# Changelog du fork LaFactory

Fork de [StJudeWasHere/seonaut](https://github.com/StJudeWasHere/seonaut), servi sur
`seo.lafactory.com`. Ce journal ne couvre que les ajouts du fork.

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
