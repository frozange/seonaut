package webgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// UserAgent identifies our downloads to Common Crawl.
var UserAgent = "LaFactory-SEOnaut-webgraph/1.0 (+https://seo.lafactory.com/bot)"

// ErrBackoff means Common Crawl answered with an error status: the caller stops
// for this run and tries again later, as Common Crawl asks.
var ErrBackoff = errors.New("webgraph: server asked to slow down")

// ErrDeadline means the time slot ended before the file was complete; the
// download resumes from the same byte next time.
var ErrDeadline = errors.New("webgraph: time slot over, download incomplete")

// Fetcher downloads whole files over a single connection, at a capped rate,
// resuming a partial file with a byte range.
type Fetcher struct {
	Client      *http.Client
	BytesPerSec int64 // 0: no cap
	Log         func(format string, args ...interface{})
}

type partMeta struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	ETag   string `json:"etag"`
	Last   string `json:"last_modified"`
	Status int    `json:"-"`
}

func (f *Fetcher) logf(format string, args ...interface{}) {
	if f.Log != nil {
		f.Log(format, args...)
	}
}

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return &http.Client{Timeout: 0}
}

// Head returns the size and version of a remote file. found is false on 404.
func (f *Fetcher) Head(ctx context.Context, url string) (meta partMeta, found bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return meta, false, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := f.client().Do(req)
	if err != nil {
		return meta, false, err
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return meta, false, nil
	case resp.StatusCode != http.StatusOK:
		return meta, false, fmt.Errorf("%w: HEAD %s: status %d", ErrBackoff, url, resp.StatusCode)
	}
	return partMeta{URL: url, Size: resp.ContentLength, ETag: resp.Header.Get("ETag"), Last: resp.Header.Get("Last-Modified")}, true, nil
}

// Fetch brings dest up to date with url before deadline. A partial file is
// kept as dest+".part", with dest+".part.json" naming the remote version it
// belongs to: a new version on the server starts the file over. It returns
// nil once dest is complete, ErrDeadline when time ran out, and ErrBackoff
// (wrapped) when the server answered with an error status.
func (f *Fetcher) Fetch(ctx context.Context, url, dest string, deadline time.Time) error {
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		return nil
	}
	part, metaPath := dest+".part", dest+".part.json"
	remote, found, err := f.Head(ctx, url)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("webgraph: %s not found", url)
	}
	var local partMeta
	if b, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(b, &local)
	}
	if local.ETag != remote.ETag || local.Size != remote.Size || local.URL != remote.URL {
		f.logf("%s: new download (%d bytes)", dest, remote.Size)
		if err := os.Remove(part); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		b, _ := json.Marshal(remote)
		if err := os.WriteFile(metaPath, b, 0o640); err != nil {
			return err
		}
	}
	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer out.Close()
	have, err := out.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if remote.Size > 0 && have > remote.Size {
		if err := out.Truncate(0); err != nil {
			return err
		}
		have, _ = out.Seek(0, io.SeekStart)
	}
	if have < remote.Size || remote.Size <= 0 {
		if err := f.get(ctx, url, out, have, remote, deadline); err != nil {
			return err
		}
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if st, err := out.Stat(); err != nil || (remote.Size > 0 && st.Size() != remote.Size) {
		return fmt.Errorf("webgraph: %s: size differs from the server's", part)
	}
	out.Close()
	if err := os.Rename(part, dest); err != nil {
		return err
	}
	os.Remove(metaPath)
	f.logf("%s: complete", dest)
	return nil
}

func (f *Fetcher) get(ctx context.Context, url string, out *os.File, have int64, remote partMeta, deadline time.Time) error {
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
		if remote.ETag != "" {
			req.Header.Set("If-Range", remote.ETag)
		}
	}
	resp, err := f.client().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ErrDeadline
		}
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0:
	case resp.StatusCode == http.StatusOK:
		if have > 0 {
			// The range was ignored: start over rather than append a second copy.
			f.logf("%s: range ignored by the server, starting over", out.Name())
			if err := out.Truncate(0); err != nil {
				return err
			}
			if _, err := out.Seek(0, io.SeekStart); err != nil {
				return err
			}
			have = 0
		}
	default:
		return fmt.Errorf("%w: GET %s: status %d", ErrBackoff, url, resp.StatusCode)
	}
	start := time.Now()
	var copied int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			copied += int64(n)
			if f.BytesPerSec > 0 {
				// Sleep until the average rate falls back under the cap.
				ahead := time.Duration(float64(copied)/float64(f.BytesPerSec)*float64(time.Second)) - time.Since(start)
				if ahead > 0 {
					select {
					case <-ctx.Done():
					case <-time.After(ahead):
					}
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil || ctx.Err() != nil {
			out.Sync()
			f.logf("%s: stopped at %d of %d bytes", out.Name(), have+copied, remote.Size)
			if ctx.Err() != nil {
				return ErrDeadline
			}
			return rerr
		}
	}
	return nil
}
