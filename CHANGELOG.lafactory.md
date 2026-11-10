# Changelog du fork LaFactory

Fork de [StJudeWasHere/seonaut](https://github.com/StJudeWasHere/seonaut), servi sur
`seo.lafactory.com`. Ce journal ne couvre que les ajouts du fork.

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
