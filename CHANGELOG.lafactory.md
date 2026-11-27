# Changelog du fork LaFactory

Fork de [StJudeWasHere/seonaut](https://github.com/StJudeWasHere/seonaut), servi sur
`seo.lafactory.com`. Ce journal ne couvre que les ajouts du fork.

## 1.12.4 - 2026-11-27

Seconde relecture indépendante du rapport de sel-tech.fr (audit 34).

- **Chaque constat dit une seule fois** : l'IA redisait les mêmes faits dans
  « L'essentiel », « Ce qui fonctionne déjà », les priorités et les cartes (les
  26 chantiers, les mots de la page piscine, le devis et le téléphone). À
  l'affichage, le rapport est lu dans l'ordre de la page et une phrase dont la
  moitié des mots principaux a déjà été dite plus haut est retirée ; un point
  fort entièrement redit disparaît. Vaut aussi pour les rapports enregistrés.
- **Plus de paragraphe de conclusion** en bas du détail technique : sans titre,
  il ne faisait que résumer le reste.
- **Fichiers d'une piste de vitesse sur toute la largeur** à 390 px (la ligne est
  une grille : `grid-column: 1 / -1`).
- Consigne de l'IA : tout « Que faire » à l'infinitif.
- Test : `TestDedupeOnTheSelTechReport` (rapport réel de l'audit 34).

## 1.12.3 - 2026-11-27

Relecture indépendante du rapport de sel-tech.fr (audit 34).

- **Plus de verdict répété** : un critère de rédaction qui est aussi une priorité
  (`e:<id>`) ne redit pas sa phrase dans sa carte ; la carte garde la question,
  la note et l'analyse. Quatre priorités sur six étaient répétées mot pour mot.
- **Pas d'étiquette de niveau quand toutes les priorités ont le même** : ni barre
  ni légende, ni « Pour finir » sur chaque carte.
- **Liens entre vos pages** : le chapeau n'annonce plus une note de 0 à 100 qui
  n'est pas affichée, et la méthode ne l'explique plus.
- **Adresses lisibles** dans le détail technique (`5000m²`, pas `5000m%c2%b2`).
- **Vitesse sur téléphone** : les fichiers d'une piste passent sur toute la
  largeur, sous la piste et son gain.
- Test : `TestReportHasNoRepeatsNorEmptyLabels`.

## 1.12.2 - 2026-11-27

- **Nom complet du fichier le plus lent** : Lighthouse abrège l'adresse dans son
  extrait (« crillon-carte-recadree-cadr… ») ; l'adresse complète est reprise
  des requêtes de la page (`crillon-carte-recadree-cadre-534x504.webp`).
- **Partage du temps du LCP retiré** (ajouté en 1.12.0) : mesuré sur une visite
  non ralentie, il additionnait 0,4 s sous une tuile LCP de 4,1 s. Le fichier en
  cause et la correction à faire restent.

## 1.12.1 - 2026-11-27

- **Corrigé : la vitesse était vide dans les audits de la 1.12.0** : une réponse
  de PageSpeed porte un `items` qui n'est pas une liste (document-latency-insight)
  et toute la réponse devenait illisible (audit 32 de sel-tech.fr). Un tel
  `items` est ignoré. Le test lit maintenant la réponse réelle complète de
  PageSpeed (gzip), pas une copie réduite.
- **Le classement du site revient dans les backlinks** (Francis : « j'ai demandé
  à retirer le charabia, pas le classement du site ») : « Votre site occupe le
  rang … sur … », et la part des sites les mieux reliés quand il en fait partie.
  L'explication sur la centralité harmonique reste retirée.

## 1.12.0 - 2026-11-27

- **Vitesse mobile, vitesse bureau** (Francis : « vitesse mobile / bureau point
  final, les gens ne sont pas si stupides ») : une carte par appareil, avec ses
  chiffres, sans les paragraphes « données sur le terrain », « données de
  laboratoire », Lighthouse ou 28 jours. Les mesures de visites réelles, quand
  Google en a, sont dans la même carte (« visites réelles »). Mesure ratée : une
  ligne.
- **Le rapport montre les solutions que PageSpeed donne** (Francis : « tu as les
  solutions sous les yeux et elles ne sont pas affichées ») : l'élément le plus
  lent à s'afficher (sur sel-tech.fr, l'image de la carte Crillon), le partage
  de son temps (réponse du serveur, attente, téléchargement, attente avant
  l'affichage), et ce qu'il faut y changer (`fetchpriority="high"` absent,
  `loading="lazy"` en trop). Sous chaque piste, les fichiers concernés. Les
  pistes ne sont plus coupées à cinq et la liste est ouverte.
- Tests : `TestParsePSIKeepsTheSolutions` (vraie réponse de PageSpeed pour
  sel-tech.fr), `TestSpeedCardShowsTheSolutions`, tests de la vitesse mis à jour.

## 1.11.6 - 2026-11-27

- **Plus de phrase répétée dans la vitesse** : sans données sur le terrain, la
  carte ordinateur redisait mot pour mot l'explication de la carte mobile. Elle
  dit maintenant « Google n'en publie pas non plus pour les visites sur
  ordinateur. »

## 1.11.5 - 2026-11-27

- **Plus de tuile « 0 ms »** (Francis : « tu as laissé le cls à 0ms ») : la
  tuile Total Blocking Time disparaît, en mobile comme en ordinateur, et l'IA
  n'en parle plus. Restent l'indice PageSpeed et le LCP en laboratoire, le LCP
  et l'INP sur le terrain.

## 1.11.4 - 2026-11-27

- **La vitesse sur ordinateur a ses propres cartes et tuiles** (Francis :
  « pourquoi tu as retiré les autres chiffres version desk ? ») : depuis la
  1.7.0, l'ordinateur tenait en une phrase sous la section (« Sur ordinateur,
  l'indice PageSpeed est de 100 sur 100 »). Il a maintenant, comme le mobile,
  une carte « Données sur le terrain, ordinateur » et une carte « Données de
  laboratoire, ordinateur » avec l'indice PageSpeed, le LCP, le TBT et les
  pistes d'amélioration. Une mesure ratée le dit dans sa carte.
- **Plus de CLS dans le rapport** (Francis : « je ne veux pas le cls ») : ni
  tuile sur le terrain, ni tuile de laboratoire, et l'IA n'en parle plus. La
  note d'ensemble des données sur le terrain reste celle de Google.
- Tests : `TestDesktopSpeedShowsItsOwnTiles`, `TestLotASectionsRenderInFrench`.

## 1.11.3 - 2026-11-27

- **`auditctl request` sur un rapport récent ne plante plus** : il renvoie le
  lien du rapport existant par courriel, mais `auditctl` ne chargeait pas les
  traductions qui écrivent ce courriel (panique sur un traducteur nul,
  sel-tech.fr). Il les charge, et sans elles le service journalise au lieu de
  planter. Test : `TestMailReportWithoutTranslationsDoesNotPanic`.

## 1.11.2 - 2026-11-27

- **Un nombre arrondi en lettres est un chiffre comme un autre** : l'IA écrivait
  « une trentaine de chantiers » pour les 26 de la page des réalisations de
  sel-tech.fr (audit 30). Le filtre qui retire une phrase dont un chiffre ne
  figure dans aucune page ne voyait que les chiffres écrits en chiffres. Il
  retire aussi « dizaine », « vingtaine », « trentaine », « centaines »,
  « milliers » (et leurs équivalents anglais) quand aucune page ne les emploie,
  et la consigne de l'IA demande le nombre exact. Test :
  `TestStripUnknownNumbersDropsRoundedCounts`.

## 1.11.1 - 2026-11-27

Faux constats relevés sur l'audit de sel-tech.fr (audit 29).

- **La limite d'un audit compte les pages, pas les images** : le robot comptait
  chaque adresse lue (images, scripts, feuilles de style) dans les 500 pages de
  l'audit. Les 46 pages de sel-tech.fr et leurs images faisaient 500 adresses :
  le rapport annonçait « 500 pages lues, limite atteinte, le site en compte
  davantage ». La limite porte désormais sur les pages HTML (`PageLimit` du
  robot), les autres fichiers ont leur propre plafond (20 par page, borné par
  `crawler.crawl_limit`). Le rapport lit le nombre de pages HTML ; le total des
  adresses reste dans `urls_crawled`. Tests : `TestPageLimit*`, `TestCrawlCaps`.
- **Une canonique écrite avec un autre encodage est la même adresse** :
  `/parking-de-5000m²/` (canonique de la page) et `/parking-de-5000m%c2%b2/`
  (lien du sitemap) désignent la même URL (RFC 3986, section 6.2.2). Le rapport
  les donnait en « pages non canoniques dans le sitemap » et les retirait des
  pages que Google peut afficher (44 au lieu de 46). L'extracteur range une
  canonique qui désigne la page elle-même sous l'écriture de la page, et les
  signaux de l'audit comparent les adresses normalisées (`urlutils.SameURL`).
  Tests : `TestSameURL`, `TestSelfCanonicalWithOtherEncoding`.
- **Durée d'exploration juste** : le début était daté par MariaDB (heure de
  Paris), la fin par Go (UTC) : chaque durée perdait une heure (-3405 s pour
  3 min 15). La fin est maintenant datée par MariaDB (`end = NOW()`).

## 1.11.0 - 2026-11-27

- **Audits lancés depuis lafactory.com par un membre connecté** : nouvelle route
  `POST /audit/relay`, appelée par le plugin `lf-audit` côté serveur, jamais par
  le navigateur. Le visiteur ne voit que lafactory.com : il se connecte ou crée
  son compte WordPress, et le site transmet l'adresse à auditer avec son e-mail.
  Le compte remplace Turnstile ; la limite par adresse IP (celle du membre), la
  garde réseau et les quotas du formulaire s'appliquent comme avant. Un
  administrateur du site lance ses audits sans quota, comme avec `auditctl`.
- Route fermée tant que `[audit] relay_secret` (32 caractères au moins) n'est pas
  posé ; le secret voyage dans l'en-tête `X-LF-Relay-Auth`, en plus de l'en-tête
  d'origine.
- `deploy/install-lafactory.sh` pose ce secret une seule fois sous `[audit]` à
  partir de `relay.secret`, rangé à côté de l'archive : le garde du compte
  refuse toute autre écriture dans le dossier de l'application.

## 1.10.9 - 2026-11-26

Passe aux huit largeurs (390 à 2560 px, Chromium et WebKit) sur la 1.10.8.

- **Vitesse côte à côte seulement quand les deux cartes ont des mesures** :
  quand Google n'a pas de données sur le terrain, cette carte n'est qu'une
  phrase. Posée à côté des quatre tuiles de laboratoire, elle laissait une
  demi-colonne vide (1024 px et plus). Les cartes s'empilent alors.
- **La pastille « Amélioration nécessaire » ne déborde plus** : elle sortait de
  sa tuile à 390 et 1024 px, et faisait défiler la page de côté sur téléphone.
  Les tuiles suivent la largeur de leur carte, pas celle de l'écran : une
  colonne sous 560 px de carte, deux jusqu'à 1100, puis des lignes pleines de
  trois ou quatre.

## 1.10.8 - 2026-11-26

- **Vocabulaire de Google imposé au rendu** : malgré la consigne donnée à
  l'IA (1.10.6), le rapport écrivait encore « titre Google », « plan du site »
  ou « description affichée par Google ». Le filtre de typographie française
  qui s'applique au rendu remet les mots de Google (« élément <title> »,
  « sitemap », « meta description ») et retire la parenthèse « (sitemap) »
  devenue inutile. Il vaut aussi pour les rapports déjà enregistrés. Ces
  tournures rejoignent la liste des phrases interdites à l'IA.

## 1.10.7 - 2026-11-26

Passe aux huit largeurs (390 à 2560 px, Chromium et WebKit) sur les rapports
de lafactory.com et bigneurons.com.

- **Une valeur ne tombe plus sous son libellé** : dans les listes (pages en
  double, pages peu reliées, conseils de vitesse, données structurées), la
  valeur a sa colonne à droite et un chemin long passe à la ligne dans la
  sienne. « texte commun à 100 % » passait sous le chemin à 820 et 900 px,
  « un seul lien reçu » à 390 px.
- **En-têtes de section sur téléphone** : le titre, puis sa barre et sa
  légende, l'un sous l'autre.

## 1.10.6 - 2026-11-26

- **Le rapport prend la largeur du site** : 1280 px au plus, comme le menu de
  lafactory.com, au lieu de 1080 (Francis : « 1080 n'a aucun sens puisque le
  template est bien plus large »). Sur un écran de 1920 px, le rapport n'était
  plus qu'une colonne étroite sous un bandeau plus large que lui.

## 1.10.5 - 2026-11-26

- **Canonique dans une autre langue, sans faux constat** : Google ne lit pas
  l'attribut `lang`. bigneurons.com publie ses articles anglais sous /big-bang/
  avec `lang="fr"` et une canonique juste vers /en/blog/ : le contrôle les
  comptait. Une page dont le titre principal (H1) est celui de sa canonique est
  une copie, pas une traduction : elle n'est plus relevée.
- **Phrase construite par langue cible** : « Vos pages en monténégrin et en
  anglais désignent une page en français comme version principale. Google
  risque de ne garder que les pages désignées et de retirer vos traductions de
  ses résultats. » Avec deux cibles, la seconde clause s'abrège (« celles en
  allemand, une page en anglais »). La 1.10.4 mélangeait les sens (« vos pages
  en monténégrin, en anglais et en français indiquent une page en français et
  en anglais »).

## 1.10.4 - 2026-11-26

- **Le sens des canoniques n'est plus confié à l'IA** : même avec les exemples
  orientés (1.10.2), elle a écrit deux fois l'inverse de la réalité sur
  bigneurons.com (« Google risque de montrer à un client français la version
  anglaise »). La conséquence de cette priorité est désormais écrite par le
  code à partir des langues mesurées : « Vos pages en anglais et en
  monténégrin indiquent une page en français comme version principale. Google
  risque de ne garder que cette page et de retirer vos traductions de ses
  résultats. »

## 1.10.3 - 2026-11-26

Disposition du rapport à toutes les largeurs (Francis : « pour les audits je
veux une disposition harmonieuse dans toutes les tailles »).

- **Plus de ligne de cartes incomplète** : sur grand écran, une grille de 4
  cartes se lit 2 + 2, de 5 cartes 3 + 2, de 7 cartes 3 + 2 + 2, au lieu
  d'une dernière ligne à une carte et deux cases vides (données structurées de
  bigneurons.com). Sur tablette, une carte en nombre impair prend toute la
  largeur ; une carte seule aussi (« Les liens entre vos pages » n'occupe plus
  une demi-largeur).
- **Priorités** : en nombre impair, la dernière prend toute la largeur.
- **Vitesse** : les deux cartes s'empilent en pleine largeur. Côte à côte, la
  carte « Vos vrais visiteurs », souvent une seule phrase, faisait le tiers de
  la hauteur de sa voisine. Les tuiles de mesure remplissent leurs lignes :
  4 sur une ligne sur grand écran, 2 + 2 sur tablette et téléphone.
- Vérifié sur les rapports de lafactory.com et bigneurons.com à 1440, 1024,
  800, 768 et 375 px : chaque ligne pleine, aucun défilement horizontal.

## 1.10.2 - 2026-11-26

- **Sens des canoniques donné à l'IA** : pour les pages dont la canonique est
  dans une autre langue, l'IA reçoit avec les exemples la langue de la page et
  celle de la page désignée. Faute de quoi elle avait écrit, sur bigneurons.com
  (audit 23), qu'une page française désignait sa version étrangère, alors que
  c'est l'inverse.
- Priorité « Pages au texte copié » renommée « Pages en double », comme la
  section du rapport.

## 1.10.1 - 2026-11-26

Relecture du rapport régénéré de bigneurons.com (audit 22).

- **Conseils de vitesse en français courant** : les titres que renvoie
  PageSpeed (« Requêtes de blocage du rendu ») sont remplacés, pour les
  conseils les plus fréquents, par un libellé clair (« Fichiers de style et
  scripts qui retardent l'affichage de la page »).
- **Une page peu reliée** : le texte passe au singulier quand une seule page
  est concernée.
- **IA** : un problème technique déjà classé en priorité n'est plus
  réexpliqué dans les analyses de rédaction et de contenu.

## 1.10.0 - 2026-11-26

Le français du rapport (Francis : « je dois faire quoi pour que tu arretes le
mauvais français et les répétitions ? », « tout ça c'est du charabia »).
Deux relectures à froid du rapport de bigneurons.com, appliquées.

- **Backlinks** : plus de rang sur 135 millions de sites, plus de colonne
  « Rang » ni de « centralité harmonique ». Restent le nombre de sites qui ont
  un lien vers le vôtre, les principaux, les nouveaux et les absents du relevé.
- **Textes fixes réécrits en français courant** : titres des contrôles sans
  « balise title », « meta description », « 30x » ni « TTFB » ; niveaux
  « À corriger d'abord / Ensuite / Pour finir » et « Grave / Important /
  Mineur » ; la vitesse, les liens internes, les pages en double, les questions
  de rédaction, la méthode.
- **Plus de répétitions** : l'avertissement « Google ne s'en sert pas pour
  classer » n'est plus redit sous chaque contrôle de la rubrique « Sans effet
  connu sur votre position dans Google » ; la phrase d'ouverture, les compteurs
  « contrôles sans défaut » et « types de problèmes », et la liste des pages
  « les plus faciles à atteindre » sont retirés.
- **Retirés du rapport** : « liens externes follow » (sa propre description dit
  qu'il n'y a rien à corriger) et « type de média incorrect » (faux positifs sur
  des polices et des vidéos de tiers).
- **IA** : un constat a une seule place dans le rapport, le conseil vit dans
  « Que faire », plus de « dans les pages lues », chaque phrase doit pouvoir se
  dire à voix haute.

## 1.9.9 - 2026-11-26

- **Auto-référence hreflang** : une page dont la canonique pointe ailleurs
  (filtres de la boutique de lafactory.com, `?lfscat=`, `?lfsprice=`) reprend
  à juste titre les versions de langue de sa page canonique, et Google ne lit
  le hreflang que sur les pages canoniques. Elle n'est plus relevée (5 faux
  constats sur l'audit 19 de lafactory.com, seule priorité technique du
  rapport).

## 1.9.8 - 2026-11-26

Relecture de l'audit 18 de bigneurons.com.

- **Chaîne de redirections** rétrogradée de critique à alerte : Google suit les
  étapes et déconseille seulement les longues chaînes, qui ralentissent
  l'exploration ([crawl budget](https://developers.google.com/search/docs/crawling-indexing/large-site-managing-crawl-budget)).
  Une chaîne de deux redirections n'est pas « à corriger d'abord ».
- **Tirets bas** : seul le chemin de l'adresse compte, pas le nom d'un
  paramètre (`?bn_lang=fr` relevé trois fois).

## 1.9.7 - 2026-11-26

- **L'outil ne bloque plus son propriétaire** (Francis : « tu trouves ça
  normal que ma propre app me bloque ? ») : `auditctl`, qui ne tourne que sur
  le serveur avec la config de l'instance, n'applique plus les quotas du
  formulaire public (3 audits par adresse sur 24 h, plafond quotidien). Ses
  audits (`ip = 'cli'`) ne comptent plus non plus dans ces quotas.

## 1.9.6 - 2026-11-26

Trois faux constats de l'audit complet de bigneurons.com (491 pages).

- **« Pages non canoniques présentes dans le sitemap »** (défaut d'origine de
  SEOnaut) : le contrôle ne regardait jamais si la page figurait au plan du
  site et relevait toute page non canonique. 59 relevées sur bigneurons.com,
  15 réellement au sitemap. Il exige maintenant `InSitemap`.
- **« Pages en HTTP »** : une image d'un serveur de préproduction, chargée en
  http, passait pour une page du site en priorité critique. Le contrôle ne
  porte plus que sur les pages HTML.
- **« URL contenant des tirets bas »** : 87 relevés, presque tous des fichiers
  de fonts.gstatic.com et des images. Le conseil de Google porte sur l'adresse
  des pages ([URL structure](https://developers.google.com/search/docs/crawling-indexing/url-structure)) :
  le contrôle ne porte plus que sur les pages HTML.

## 1.9.5 - 2026-11-26

- **Plafond de pages atteint, enfin dit** : l'audit de bigneurons.com s'est
  arrêté au plafond de 150 pages, mais une page que l'analyseur HTML n'a pas pu
  lire (plus de 512 éléments imbriqués) n'est jamais enregistrée : 149 pages,
  donc « plafond non atteint », et le rapport présentait l'échantillon comme le
  site entier (7 canoniques fautives vues sur 58). Le plafond compte comme
  atteint à 1 % près (une page au moins), et la phrase d'ouverture du rapport
  le dit : « à partir des 149 premières pages explorées : le site en compte
  davantage ».

## 1.9.4 - 2026-11-26

Défauts relevés sur le rapport de bigneurons.com (Francis : « Décalé », « on
croirait à un gag, ce sont des backlinks », « plein d'autres erreurs »).

- **Section « Backlinks »** : « Popularité : les sites qui vous citent »
  devient « Backlinks : les sites qui font un lien vers le vôtre » ; le mot
  « citer » disparaît des libellés, de la comparaison aux concurrents et de la
  méthode (FR et EN).
- **Rang décalé** : un nom de domaine long poussait le rang sur une seconde
  ligne. Une ligne qui porte un rang ne passe plus à la ligne ; le nom se coupe.
- **Nouveau contrôle, canonique dans une autre langue**
  (`ERROR_CANONICAL_OTHER_LANGUAGE`, critique) : une page dont la canonique
  pointe vers une page d'une autre langue (`lang` de base différent, `fr-FR` et
  `fr` restent égaux). Google demande une canonique dans la même langue
  ([versions localisées](https://developers.google.com/search/docs/specialty/international/localized-versions)).
  Sur bigneurons.com, 58 pages anglaises et monténégrines désignaient l'accueil
  français ; l'ancien rapport n'en voyait que 8, par le biais du plan du site.
- **Redirections (30x) rétrogradées** de critique à avertissement : Google suit
  les redirections sans pénalité ; la description ne parle plus de « budget de
  crawl », qui ne concerne que les très grands sites
  ([crawl budget](https://developers.google.com/search/docs/crawling-indexing/large-site-managing-crawl-budget)).
- **Hors classement Google** : le TTFB mesuré par notre robot (Google classe sur
  les vraies visites, [Core Web Vitals](https://developers.google.com/search/docs/appearance/core-web-vitals))
  et le type de média des ressources (aucune règle Google ; faux positif sur
  `fonts.googleapis.com/css2`, adresse sans extension prise pour du HTML). Les
  descriptions des images volumineuses et sans dimensions ne disent plus
  qu'elles nuisent au classement.
- **Faux « Brief Brief »** : un bouton qui porte un libellé bureau et un
  libellé mobile (l'un masqué en CSS) était lu deux fois par l'IA, qui en
  tirait une réécriture. Un libellé fait du même texte répété est lu une fois.

## 1.9.3 - 2026-11-26

Mise en page du rapport (Francis : « il faut deux colonnes, et le responsif en
général est mauvais »).

- **Priorités sur deux colonnes** dès que le rapport dispose de 680 px (tablette
  et ordinateur) ; le numéro passe dans le coin de la carte au lieu d'occuper
  une gouttière de 56 px, et l'encadré « Que faire » s'aligne en bas des deux
  cartes d'une même ligne.
- **Colonnes calculées sur la largeur réelle du rapport** (requêtes de
  conteneur) et non plus sur celle de la fenêtre : une colonne sur téléphone,
  deux sur tablette, trois sur grand écran quand il y a au moins trois cartes.
  Une carte seule en fin de grille à deux colonnes prend toute la largeur. Sur
  tablette (768 px), tout passait sur une seule colonne.
- **Largeur** : la page du rapport ne garde plus 80 % de la fenêtre (réglage du
  conteneur Divi) ; elle prend la largeur moins une gouttière de 16 à 48 px,
  1080 px au plus. 707 px utiles à 768 au lieu de 614.
- Détail technique : le « + » d'un problème ne passe plus seul à la ligne.

## 1.9.2 - 2026-11-25

Défauts relevés par une relecture du rapport de lafactory.com.

- **Formulaires vus par l'IA** : l'extraction du texte saute les formulaires,
  si bien qu'une page Contact avec nom, e-mail, message et captcha passait
  pour vide (« la page Contact ne contient presque rien », six fois dans le
  rapport). Chaque page lue transmet maintenant ses formulaires (champs
  visibles, bouton), et la consigne dit qu'une page de contact se juge à son
  formulaire et à ses coordonnées, jamais à son nombre de mots.
- **Versions en d'autres langues** : les liens hreflang de chaque page sont
  transmis à l'IA, qui ne peut plus écrire qu'elle n'a pas vérifié si une
  version française existe quand la page la déclare.
- Recherches visées : 2 à 4 mots, comme un client les tape, dans la langue
  de la page et non dans celle du rapport (elles étaient en français pour des
  pages anglaises, et Google Suggest était interrogé dans la langue de la
  page).
- **Typographie française** : espace insécable avant « : » et « » », après
  « « », espace fine insécable avant « ; ! ? % », sur tout le texte du
  rapport, textes de l'IA compris (le balisage, les styles et les scripts ne
  sont pas touchés). Un deux-points ne commence plus une ligne.
- Textes : « 1 type de problème relevé » (singulier), « Liens non retrouvés ce
  trimestre » au lieu de « Plus vus ce trimestre », « des 3 % de sites les
  mieux reliés de cet échantillon » au lieu de « du web », « Aucune page
  isolée » au lieu de « Toutes reliées » au-dessus de « Pages peu reliées »,
  notes toujours à une décimale (« 8,0 » à côté de « 9,9 »).

## 1.9.1 - 2026-11-25

- **Rapport plus lisible sur toutes les largeurs** : les grilles ne laissent
  plus de colonne vide (`auto-fit`) ; sur téléphone, le rapport prend toute la
  largeur moins 16 px au lieu des 10 % de marge du thème, les trois notes
  tiennent en trois lignes compactes, les chiffres clés restent en 2 x 2 (4 en
  ligne dès 700 px) et les volets ont une zone de clic de 44 px.
- Lignes des listes : le chiffre reste sur la ligne du libellé et ne passe
  dessous que si les deux ne tiennent pas (avant : toujours dessous sur
  téléphone).
- **Popularité** : la carte du nombre de sites et du rang prend toute la
  largeur, les trois listes se partagent la ligne du dessous ; le mot « rang »
  n'est plus répété à chaque ligne, une ligne d'en-tête « Site / Rang » le dit
  une fois (aussi dans la liste des sites qui citent vos concurrents).
- Vitesse : tuiles de mesure en 2 x 2, seuil sur une ligne, pastilles alignées.
- Corrigé : les cartes « Ce qui fonctionne déjà » n'avaient aucune marge
  intérieure et plusieurs marges (notes sous les cartes, aide sous les titres)
  valaient 0, écrasées par les remises à zéro du gabarit, désormais en
  `:where()`.

## 1.9.0 - 2026-11-25

- **Adresses laissées de côté à la demande du propriétaire** : une liste par
  site, `/srv/seonaut-exclude/<nom-du-site>.txt` (`[audit] exclude_dir`), une
  adresse complète, un chemin ou un préfixe `/chemin/*` par ligne. Le robot ne
  les demande jamais et ne suit pas leurs liens, plan du site compris. Le
  rapport le dit dans la méthode (« %1% adresses ont été laissées de côté »),
  et l'IA reçoit la même information pour ne pas en tirer de constat.
  Premier usage : le blog de lafactory.com (602 articles et leurs étiquettes),
  dont les articles sont à la racine comme les pages.
- `auditctl cancel <id>` : marque un audit en file ou interrompu comme
  annulé, pour qu'un redémarrage ne le relance pas.

## 1.8.2 - 2026-11-24

- Graphe Common Crawl : un refus du serveur (« 503 SlowDown », revenu deux fois
  le soir du premier téléchargement) est retenté deux fois, à une minute
  d'intervalle, dans la même fenêtre, avant la pause de 45 min. Avant, un seul
  refus faisait perdre la fenêtre de l'heure.

## 1.8.1 - 2026-11-24

- Popularité : la seconde carte de chaque rangée n'est plus décalée de 20 px
  (l'espacement ne vise plus que les blocs empilés de la section).

## 1.8.0 - 2026-11-24

Lot B de l'étude des outils SEO : la popularité, lue dans le graphe des liens
entre domaines que Common Crawl publie chaque trimestre (gratuit, données
libres). Aucune métrique inventée : pas de « liens toxiques », pas de clone de
Domain Authority, pas de Spam Score.

- **Rapport, section « Popularité : les sites qui vous citent »** (après la
  vitesse) : nombre de sites qui citent le site audité, les dix mieux reliés,
  gagnés et perdus depuis le trimestre précédent, et le rang de centralité
  harmonique de Common Crawl présenté comme un repère de LaFactory (Google
  publie seulement qu'il se sert des liens, « link analysis systems », sans
  note). Un domaine absent de l'échantillon affiche « Pas encore dans
  l'échantillon », jamais 0. Note visible : Common Crawl est un échantillon,
  les chiffres sont plus bas que dans les outils payants.
- **« Vous face aux concurrents »**, seulement quand des concurrents sont
  connus (`auditctl -c config popularity <nom> <concurrent> ...`) : les mêmes
  chiffres pour chacun, et les sites qui citent au moins deux concurrents (ou
  le seul donné) et pas le site.
- La popularité entre au dossier de preuves de l'IA, avec la mention
  d'échantillon ; elle ne crée aucune priorité. Consigne de l'IA : jamais de
  lien « toxique », jamais de note d'autorité ; tournures bannies ajoutées.
- **`internal/webgraph` et la commande `webgraph`** : découverte de la
  dernière publication (une série de HEAD, une fois par jour), téléchargement
  de `vertices`, `edges` et `ranks` (environ 11 Go) sur une seule connexion,
  plafonné à 6 Mo/s, repris par plage d'octets, pause de 45 min sur toute
  réponse d'erreur ; puis index sur disque lu par `mmap` (noms triés, rangs,
  décalages et sources des liens entrants, environ 12 Go), en deux passes et
  par paquets pour tenir en mémoire. Deux trimestres gardés (le dernier et
  celui d'avant, pour les gagnés et perdus), les plus anciens supprimés.
- Données dans `/srv/seonaut-webgraph` (compte lafactor, hors de `/home`, donc
  hors des sauvegardes Backuply quotidiennes ; `CACHEDIR.TAG` le sort de
  l'image Borg). Outil en `/usr/local/bin/seonaut-webgraph`, planifié par
  `/etc/cron.d/seonaut-webgraph` (`deploy/webgraph-cron.sh`) : téléchargement
  de la minute 41 à la minute 04 seulement, jamais à 1 h, jamais pendant une
  commande Common Crawl de Sestaro (lots à la minute 07, miroir à 01:41, même
  IP ; Common Crawl a déjà bloqué srv pour excès le 2026-09-25). Une fois un
  trimestre indexé, `auditctl popularity -missing` ajoute la section aux
  rapports qui ne l'ont pas.
- `[audit] webgraph_dir` (défaut `/srv/seonaut-webgraph`, vide pour couper).

## 1.7.4 - 2026-11-24

- Liens internes : seule une page qu'aucune autre page ne relie (trouvée au mieux
  par le plan du site) crée une priorité. Google documente la découverte : un
  seul lien explorable suffit pour qu'il trouve une page. Les pages reliées une
  seule fois restent listées, à titre d'information, sans priorité.
- `LinkingReport.Orphans` compte ces pages isolées ; la carte du rapport les
  annonce et passe à « Pages isolées » seulement dans ce cas.
- `auditctl requery` retire aussi d'un rapport enregistré les priorités
  mesurées que les règles actuelles ne créent plus (sur sel-tech.fr :
  « Pages peu reliées », 4 pages reliées une fois).

## 1.7.3 - 2026-11-24

Les mesures du lot A relues contre la documentation de Google Search Central
(règle de Francis : un outil SEO qui invente un problème pour vendre sa
correction n'entre pas dans le rapport).

- Données structurées alignées sur les tableaux « obligatoire / conseillé » des
  pages de Google : le nom d'une Organization devient conseillé (Google n'y
  exige rien), l'image n'est plus réclamée à un LocalBusiness (absente de la
  liste de Google). Le reste (Product, Offer, AggregateOffer, livraison,
  retours, avis, fil d'Ariane, FAQ) était déjà conforme.
- Liens internes : présentés comme la facilité avec laquelle visiteurs et robot
  de Google atteignent une page (Google dit trouver les pages en suivant les
  liens), « repère de LaFactory, pas une note de Google ». Plus de « donne de
  l'importance aux yeux de Google » ni de « calcul rendu célèbre par Google ».
  La ligne « pages à 4 clics ou plus » sort du rapport et du dossier de l'IA :
  Google ne documente aucune règle de profondeur.
- Pages copiées : priorité ramenée à « Optimisation » (Google choisit une page
  parmi des doublons, il ne sanctionne pas).

## 1.7.2 - 2026-11-24

- **Plus de « pages qui se font concurrence »** (Francis : la cannibalisation est
  un mythe ; Google montre plusieurs résultats d'un même site, et traiter un
  sujet en expert demande plusieurs pages dont les mots se recoupent). Retirés :
  la comparaison titre, description et H1 (`CompetingPages`), les paires de pages
  à qui l'IA prête la même recherche (`SameQueryPairs`), la priorité
  `m:competing`, la carte du rapport et ses textes. Le poids LibreCrawl de cette
  comparaison sort de `THIRD_PARTY_NOTICES`.
- Seules restent les **pages copiées** : texte principal presque mot pour mot le
  même (85 % de suites de cinq mots communes), traductions exclues. La section
  s'appelle « Les pages copiées » et ne parle jamais de concurrence.
- La consigne de l'IA interdit de dire que des pages du site se font concurrence ;
  « cannibalisation », « se font concurrence », « se concurrencent »,
  « cannibalisent » et « concurrence interne » entrent dans les tournures
  bannies. La description du contrôle d'origine des titres en double ne dit plus
  que les pages « se concurrencent » : elle reprend la recommandation de Google
  (un titre propre à chaque page).
- Vitesse : une espace insécable lie chaque chiffre à son unité (« 2,5 s » ne se
  coupe plus) ; deux mesures par ligne dans les cartes de vitesse.

## 1.7.1 - 2026-11-24

Corrections vues sur le premier rapport réel en 1.7.0 (sel-tech.fr).

- Une recherche écartée dit ce que Google propose à la place : jusqu'à trois
  suggestions qui partagent au moins deux mots utiles avec elle (sur sel-tech.fr,
  4 recherches sur 6 étaient écartées sans autre indication).
- `auditctl requery <nom ou jeton>` refait la vérification Google Suggest d'un
  rapport terminé, sans exploration ni appel à l'IA.
- Vitesse : les durées gardent leur décimale (« 4,0 s », plus « 4 s »), la
  stabilité deux décimales ; l'aide ne répète plus l'adresse « / » de la page
  d'accueil ; deux mesures par ligne sur téléphone, cartes alignées en haut.
- Les listes de pages passent sur une colonne sur téléphone (le gain d'une piste
  de vitesse ne comprime plus son titre).
- La ligne « autres types trouvés » (WebPage, WPHeader...) est retirée du
  rapport : du vocabulaire technique sans action pour le lecteur.

## 1.7.0 - 2026-11-24

Lot A de l'étude des outils libres (mémoire lafactory, 2026-11-24) : quatre mesures
nouvelles, toutes gratuites, et les recherches visées confrontées à Google. Le
rapport suit l'ordre « juste milieu » de l'étude : synthèse, priorités, points
forts, recherches, résultats enrichis, vitesse, liens entre les pages, pages en
concurrence, rédaction, contenu, détail technique.

- Le robot garde de chaque page ce que la base ne stocke pas (données
  structurées, empreinte du texte principal, liens suivis) pendant l'exploration
  (`audit.Collector`, branché sur `CrawlerHandler` par `PageObserver`) : aucune
  seconde visite du site.
- **Vitesse réelle** (`perf.go`, portage en Go de `audit-seo/audit/perf.py`) :
  API PageSpeed Insights, téléphone et ordinateur, données de vrais visiteurs
  (CrUX, page sinon site entier) séparées de la simulation Lighthouse. La clé est
  lue par son nom (`[audit] psi_key_file`, `psi_key_name`, défaut `PSI_API_KEY`),
  envoyée dans un en-tête, jamais dans l'adresse ni dans un message. Un appel raté
  n'enregistre aucun chiffre.
- **Données structurées** (`structured.go`, `schema_rules.go`) : lecture JSON-LD
  et microdonnées par `astappiev/microdata` (BSD-2), règles des résultats enrichis
  portées d'`adobe/structured-data-validator` (Apache-2.0) pour Product, Offer,
  AggregateOffer, AggregateRating, Review, BreadcrumbList, Organization, livraison
  et retours, plus LocalBusiness, PostalAddress et FAQPage d'après la
  documentation de Google. Blocs JSON-LD illisibles comptés.
- **Liens entre les pages** (`linkscore.go`, portage du Link Score de LibreCrawl,
  MIT) : PageRank interne de 0 à 100, redirections suivies, pages les plus mises
  en avant, pages reliées par une page ou aucune (fiches produits et pages clés
  d'abord), pages à 4 clics ou plus.
- **Doublons et pages en concurrence** (`duplicates.go`) : textes principaux
  presque identiques (empreintes de suites de cinq mots, similarité estimée de
  85 % ou plus) ; paires au titre, à la description et au H1 semblables (poids de
  LibreCrawl, seuil 0,85) ; pages auxquelles l'IA prête la même recherche.
  Traductions (langues différentes ou liens hreflang), pages canonisées ailleurs,
  `noindex` et pagination exclues.
- **Recherches visées vérifiées** (`suggest.go`) : chaque recherche devinée par
  l'IA est confrontée à Google Suggest ; gardée si Google la propose ou propose
  une forme proche (un mot de différence), écartée sinon et listée à part dans
  le rapport ; marquée « non vérifiée » si Suggest ne répond pas.
- Plan d'action : vitesse réelle mauvaise ou moyenne, fiches produits en erreur,
  doublons, pages en concurrence et pages peu reliées entrent dans les priorités ;
  la simulation seule n'en crée jamais. Les nouvelles mesures entrent dans le
  dossier de preuves de l'IA, qui a une règle de plus sur la vitesse et les FAQ.
- Styles ajoutés à la feuille du gabarit (`lfr_style`), pas par-dessus.
  `THIRD_PARTY_NOTICES` et `licenses/` portent les licences reprises.
- Les rapports enregistrés avant 1.7.0 s'affichent comme avant.

## 1.6.4 - 2026-11-24

- Retour à la sélection des pages clés de 1.6.2 : la 1.6.3 (lecture des pages
  `noindex`) visait dev.lafactory.com, un banc de test que Francis ne considère
  pas comme une cible d'audit. Retirée sur sa demande.

## 1.6.2 - 2026-11-24

- Le texte de l'IA parle comme le patron du site, pas comme un consultant : la
  consigne interdit le jargon (message, cible, preuves, appels à l'action,
  positionnement, proposition de valeur...) et donne les mots simples à la place.
  Le résumé et la conclusion disent concrètement ce qui est bien, ce qui manque et
  par quelles pages commencer ; quand le texte des pages n'a pas pu être lu, ils
  disent ce qui a été lu, ce qui n'a pas pu être jugé et la suite. La conclusion
  de sel-tech.fr, validée par Francis, sert d'exemple de ton.
- Les domaines et adresses ne sont plus coupés (« dev. lafactory. com ») : le
  filtre des chiffres découpait les phrases à chaque point. Une phrase finit
  maintenant sur « . », « ! » ou « ? » suivis d'une espace ou de la fin.

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
