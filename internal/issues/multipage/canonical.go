package multipage

import (
	"github.com/stjudewashere/seonaut/internal/issues/errors"
	"github.com/stjudewashere/seonaut/internal/models"
)

// Creates a MultipageIssueReporter object that contains the SQL query to check for pages
// that are canonicalized to non-canonical pages.
func (sr *SqlReporter) CanonicalizedToNonCanonical(c *models.Crawl) *models.MultipageIssueReporter {
	query := `
		SELECT
			a.id
		FROM pagereports AS a
		INNER JOIN pagereports AS b ON a.url = b.canonical
		WHERE a.crawl_id = ?
			AND b.crawl_id = ?
			AND a.canonical != ""
			AND a.canonical != a.url
			AND a.crawled = 1
			AND b.crawled = 1`

	return &models.MultipageIssueReporter{
		Pstream:   sr.pageReportsQuery(query, c.Id, c.Id),
		ErrorType: errors.ErrorCanonicalizedToNonCanonical,
	}
}

// Creates a MultipageIssueReporter object that contains the SQL query to check for pages
// that are canonicalized to non-indexable pages.
func (sr *SqlReporter) CanonicalizedToNonIndexable(c *models.Crawl) *models.MultipageIssueReporter {
	query := `
		SELECT id
		FROM pagereports
		WHERE
			crawl_id = ? AND noindex = 1
		AND canonical IN (
			SELECT url
			FROM pagereports
			WHERE 
			crawl_id = ? AND canonical != "" AND canonical != url
		)`

	return &models.MultipageIssueReporter{
		Pstream:   sr.pageReportsQuery(query, c.Id, c.Id),
		ErrorType: errors.ErrorCanonicalizedToNonIndexable,
	}
}

// Creates a MultipageIssueReporter object that contains the SQL query to check for pages
// that are canonicalized to redirects.
func (sr *SqlReporter) CanonicalizedToRedirect(c *models.Crawl) *models.MultipageIssueReporter {
	query := `
		SELECT
			pr.id
		FROM pagereports AS pr
		INNER JOIN pagereports AS pr2 ON pr.canonical = pr2.url
		WHERE pr.crawl_id = ?
			AND pr2.crawl_id = ?
			AND pr.canonical != pr.url
			AND pr2.status_code >= 300
			AND pr2.status_code < 400;`

	return &models.MultipageIssueReporter{
		Pstream:   sr.pageReportsQuery(query, c.Id),
		ErrorType: errors.ErrorCanonicalizedToRedirect,
	}
}

// Creates a MultipageIssueReporter object that contains the SQL query to check for pages
// that are canonicalized to error pages with status code in the 40x and 50x range.
func (sr *SqlReporter) CanonicalizedToError(c *models.Crawl) *models.MultipageIssueReporter {
	query := `
		SELECT
			pr.id
		FROM pagereports AS pr
		INNER JOIN pagereports AS pr2 ON pr.canonical = pr2.url
		WHERE pr.crawl_id = ?
			AND pr2.crawl_id = ?
			AND pr.canonical != pr.url
			AND pr2.status_code >= 400;`

	return &models.MultipageIssueReporter{
		Pstream:   sr.pageReportsQuery(query, c.Id),
		ErrorType: errors.ErrorCanonicalizedToError,
	}
}

// Creates a MultipageIssueReporter object that contains the SQL query to check for pages
// canonicalized to a page written in another language. Google asks for a canonical in the
// same language: a translation pointing to the original gets dropped from the results.
// https://developers.google.com/search/docs/specialty/international/localized-versions
func (sr *SqlReporter) CanonicalizedToOtherLanguage(c *models.Crawl) *models.MultipageIssueReporter {
	query := `
		SELECT
			a.id
		FROM pagereports AS a
		INNER JOIN pagereports AS b ON a.canonical = b.url
		WHERE a.crawl_id = ?
			AND b.crawl_id = ?
			AND a.canonical != ""
			AND a.canonical != a.url
			AND a.crawled = 1
			AND b.crawled = 1
			AND a.lang != ""
			AND b.lang != ""
			AND LOWER(SUBSTRING_INDEX(REPLACE(a.lang, "_", "-"), "-", 1)) != LOWER(SUBSTRING_INDEX(REPLACE(b.lang, "_", "-"), "-", 1))`

	return &models.MultipageIssueReporter{
		Pstream:   sr.pageReportsQuery(query, c.Id, c.Id),
		ErrorType: errors.ErrorCanonicalOtherLanguage,
	}
}
