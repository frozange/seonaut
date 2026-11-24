# Changelog du fork LaFactory

Fork de [StJudeWasHere/seonaut](https://github.com/StJudeWasHere/seonaut), servi sur
`seo.lafactory.com`. Ce journal ne couvre que les ajouts du fork.

## 1.6.1 - 2026-11-24

- Le script de déploiement sur srv entre dans le dépôt (`deploy/install-lafactory.sh`,
  copié dans `/root/claude/seonaut-deploy/install.sh`). Toute écriture dans le
  compte (archive dépaquetée, binaires, `config`, `run.sh`, crontab, redémarrage)
  est faite par `lafactor` lui-même via `sudo -u lafactor` : root ne fait que lire
  l'archive et les secrets, et ne suit donc aucun lien posé dans le compte.

## 1.6.0 - 2026-11-24

Les adresses de rapport portent le nom du site : le jeton de 43 caractères
faisait bloquer le lien par Cloudflare chez des visiteurs.

- `audit.Slug` : l'hôte du site audité en minuscules, chaque suite de points ou
  d'autres signes changée en un tiret (`sel-tech.fr` donne `sel-tech-fr`). Pas de
  colonne en base : le nom se calcule à partir de l'adresse du site.
- Un nom de site désigne l'audit terminé le plus récent de ce site (le plus
  récent tout court quand aucun n'est terminé).
- `[audit] report_url` accepte `{slug}` en plus de `{token}`. E-mails, formulaire
  et anciennes adresses `/rapport/<jeton>` mènent à l'adresse au nom du site.
- `GET /rapport/<jeton ou nom>/fragment` accepte les deux et renvoie le nom
  canonique dans l'en-tête `X-LF-Audit-Slug`, pour que la page du site redirige
  les anciens liens. `auditctl status` accepte aussi le nom.
- `VERSION` était resté à 1.4.2 en 1.5.0 ; il suit de nouveau ce journal.

## 1.5.0 - 2026-11-24

Le rapport d'audit s'affiche dans lafactory.com, comme une page du site, et sa
mise en page est refaite.

- Nouvelle adresse `GET /rapport/{token}/fragment` : le rapport, ou l'état d'un
  audit en cours, en fragment HTML (ni `html`, ni `head`, ni `h1`, ni appel à
  l'action), styles limités à `.lfr` et tailles prises dans les variables
  `--lf-*` du site. Les en-têtes `X-LF-Audit-Status` et `X-LF-Audit-Host`
  renseignent la page qui l'encadre. Tout texte lu sur le site audité ou écrit
  par l'IA reste échappé par `html/template`.
- Réglage `[audit] report_url` (`{token}` remplacé) : quand il est posé, les
  e-mails, le formulaire et les anciennes adresses `/rapport/{token}` mènent à
  cette page du site (302). Vide, rien ne change.
- Mise en page : jauges circulaires pour les trois notes, bandeau de chiffres
  (pages lues, contrôles sans défaut, problèmes, priorités), barres de
  répartition des priorités et des critères, une barre par problème technique
  à la mesure des pages concernées, recherches visées en cartes avec un
  indicateur de correspondance, réécritures en « aujourd'hui / proposition ».
  Le détail des critères, les exemples, les relevés hors classement et la
  méthode se replient. Chaque état porte une icône et un mot, jamais la
  couleur seule. Plus de dégradé violet ; bouton de la charte (ivoire).

## 1.4.2 - 2026-11-11

- La consigne de l'IA rangeait E-E-A-T parmi les idées reçues. Google dit que
  E-E-A-T n'est pas un facteur de classement en soi, mais que ses systèmes
  cherchent ces qualités à travers un ensemble de signaux ([helpful
  content](https://developers.google.com/search/docs/fundamentals/creating-helpful-content)) :
  la consigne le décrit désormais ainsi.

## 1.4.1 - 2026-11-11

Le rapport de lafactory.com affirmait trois choses fausses, toutes nées de la
lecture des pages par SEOnaut, pas du site :

- « Du code apparaît dans le texte du pied de page » : le texte du pied de page
  reprenait le contenu des balises `<style>` et `<script>` (la feuille de style du
  bouton WhatsApp), qu'aucun navigateur n'affiche.
- « La FAQ pèse près de 39 000 mots » : le compteur de mots d'origine comptait le
  CSS des balises `<style>`. Il ignore désormais `style`, `noscript` et `template`
  comme il ignorait déjà `script`.
- « Case Studies répète chaque titre deux fois » : les copies des onglets inactifs,
  marquées `hidden`, étaient lues comme du texte visible. Les éléments `hidden` ou
  `aria-hidden="true"` ne sont plus lus.

## 1.4.0 - 2026-11-11

La note technique ne compte plus que ce que Google dit utiliser. Les contrôles
d'origine qui mesurent autre chose passent dans une rubrique « Hors classement
Google » du rapport, sans poids sur la note ni place dans les priorités :

| contrôles | ce que dit Google |
|---|---|
| ordre des titres, absence de H1 | « number and order of headings » figure dans « Things we believe you shouldn't focus on » ([guide de démarrage SEO](https://developers.google.com/search/docs/fundamentals/seo-starter-guide)) |
| peu de contenu | la longueur seule ne compte pas pour le classement (même guide) |
| title court ou long | aucune limite de longueur ([title links](https://developers.google.com/search/docs/appearance/title-link)) |
| description courte ou longue | aucune limite de longueur ([snippets](https://developers.google.com/search/docs/appearance/snippet)) |
| liens externes sans nofollow | un lien ordinaire n'a besoin d'aucun attribut rel ([qualify outbound links](https://developers.google.com/search/docs/crawling-indexing/qualify-outbound-links)) |
| attribut lang absent ou invalide | Google n'utilise pas l'attribut lang ([sites multirégionaux](https://developers.google.com/search/docs/specialty/international/managing-multi-regional-sites)) |
| trop de liens, alt trop long | aucune règle chez Google |
| ID dupliqués | « the web in general is not valid HTML » (guide de démarrage) |
| taille du DOM, images sans dimensions ou lourdes | la vitesse et la stabilité se mesurent sur de vraies visites ([Core Web Vitals](https://developers.google.com/search/docs/appearance/core-web-vitals)) |
| en-têtes HSTS, CSP, X-Content-Type-Options | sécurité, pas classement |

Les descriptions de ces contrôles ne promettent plus d'effet sur Google, et l'IA
du rapport a pour consigne de ne jamais les présenter comme facteurs de classement.

## 1.3.3 - 2026-11-11

- Longueur des titres et des descriptions comptée en caractères, plus en octets :
  un accent comptait double, et un titre français de 60 caractères passait pour
  trop long.

## 1.3.2 - 2026-11-11

Contrôles corrigés après le premier audit réel de lafactory.com, qui relevait trois
faux défauts :

- Langue hreflang : une page en `en-US` qui se déclare `hreflang="en"` est
  cohérente. La langue doit correspondre, la région ne compte que si les deux
  balises en donnent une (25 pages faussement signalées).
- HSTS : seules les pages HTML sont contrôlées. Les feuilles de style, polices et
  images servies par le cache de Cloudflare comptaient comme des pages (124).
- Images sans `alt` : seule l'absence de l'attribut est signalée. `alt=""` est la
  norme d'accessibilité (WCAG H67) pour une image décorative.

Et :

- Au démarrage, l'audit supprime les projets du robot laissés par un audit
  interrompu (redémarrage, panne).
- `auditctl -fresh request` relance un crawl même si un rapport récent du même site
  existe.

## 1.3.1 - 2026-11-11

- Les données d'un crawl interrompu se suppriment en arrière-plan au démarrage. La
  suppression procède par lots ralentis : après l'interruption d'un crawl de 3 300
  pages, le serveur restait plusieurs minutes sans écouter.

## 1.3.0 - 2026-11-11

- Plafond de pages propre à l'audit (`audit.max_pages`, 150 par défaut). Le premier
  audit de prod suivait la limite de l'instance (5 000 pages) et dépassait les
  45 minutes allouées au crawl.
- Porte de test du propriétaire pour Turnstile (`turnstile.qa_bypass`) : un secret
  d'au moins 32 caractères, envoyé à la place de la réponse Turnstile, passe sans
  appeler Cloudflare, comparé en temps constant. Vide par défaut, donc fermée.

## 1.2.0 - 2026-11-11

- Commande `auditctl` : demande d'audit depuis le serveur (`request <site> <e-mail>`)
  et état d'un audit (`status <jeton>`). Elle passe par la même validation et les
  mêmes limites que le formulaire, sans le navigateur ni la vérification humaine :
  c'est la voie du propriétaire pour les tests et les demandes faites pour un tiers.
  Elle exige la config de l'instance, donc le compte qui fait tourner SEOnaut.

## 1.1.0 - 2026-11-11

Audit gratuit sur demande, complété par l'IA, avec une page de rapport à la charte LaFactory.

- Formulaire `/audit` (adresse e-mail, Turnstile, limites par jour, par adresse et
  par IP, rapport réutilisé 24 h pour un même site) et file d'attente traitée un
  audit à la fois.
- Crawl par le robot de SEOnaut, puis dossier de preuves : faits du site, problèmes
  avec exemples, texte des pages principales (blog et listes exclus), pied de page,
  boutons et contacts.
- Analyse IA en trois passes (analyse, rédaction, relecture adverse) par une
  passerelle en ligne de commande, suivies de contrôles déterministes : chiffres
  absents des preuves retirés, références internes retirées du texte, réécritures
  vérifiées. Sur seo.lafactory.com, la passerelle est `srv-claude`, commune à tout
  le serveur.
- Notes publiées : technique, rédaction, contenu, avec un barème affiché.
- Rapport `/rapport/<jeton>` : notes, synthèse, priorités, points forts, critères,
  requêtes visées (en cartes sur mobile), réécritures, détails techniques, méthode.
  Sans IA, le rapport est publié avec les mesures seules et le dit.
- Interface en français complet.

## 1.0.0 - 2026-11-09

Protections nécessaires pour ouvrir l'outil au public.

- Garde réseau du crawler (`internal/netguard`) : refus à la connexion des adresses
  privées, réservées, de bouclage, de métadonnées cloud et des IP du serveur, ports
  80 et 443 seulement, sans suivre les proxys d'environnement. L'index des sitemaps
  passe désormais par le client du crawler au lieu de `http.Get`.
- URL de projet vérifiée à la création, message clair si elle est refusée.
- Turnstile sur l'inscription et la connexion, bouton bloqué tant que le jeton
  n'est pas reçu.
- Vérification de l'adresse e-mail à l'inscription : lien valable 24 h, seul le
  hachage est stocké, comptes jamais vérifiés purgés. Envoi par `sendmail` local ou
  SMTP (`tls_server_name` pour un relais joint par IP).
- Limites : inscriptions par IP, tentatives de connexion par IP et par adresse,
  e-mails de vérification par adresse, crawls par jour et par utilisateur, crawls
  simultanés, projets par utilisateur, pages par crawl.
- Options désactivables : user-agent personnalisé, crawl qui ignore robots.txt.
- En-têtes : `X-Robots-Tag` noindex, `robots.txt` en Disallow total, HSTS,
  nosniff, `X-Frame-Options`. Cookies `Secure` et `SameSite=Lax`, clés de session
  persistantes.
- Verrou d'origine : en-tête secret injecté par Cloudflare, 403 sans lui.
- Suppression de projet en POST (elle était en GET, donc déclenchable par un lien).
- IP réelle du visiteur lue dans `CF-Connecting-IP` seulement derrière un proxy de
  confiance.
