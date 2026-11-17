# Journal des changements

## v1.1.0 - 17 novembre 2026

Cette version succède à v1.0.1. Elle regroupe une nouvelle fonctionnalité
d'export XLSX et des corrections de reconnaissance des métadonnées principales.
La documentation anglaise ajoutée depuis le tag v1.0.1 est également actualisée.

### Choix du numéro de version

- **Majeur : inchangé (1).** Aucune commande existante n'est supprimée et le
  format JSON/gzip de l'index reste en version 1. Le finder continue à le lire.
- **Mineur : 0 → 1.** La commande `-manifest` ajoute un export XLSX autonome.
- **Patch : remis à 0.** Les corrections sont incluses dans cette version
  mineure ; leur nombre n'est pas ajouté au numéro de patch.

Le numéro retenu est donc **v1.1.0**, et non v1.0.2 ou v2.0.0. La politique
d'extraction fournie change néanmoins volontairement les résultats : cette
évolution et la réindexation nécessaire doivent accompagner le déploiement.
Ce document ne crée ni tag ni Release GitHub.

### Fonctionnalité ajoutée : manifest XLSX

- Nouvelle option de l'indexeur : `-manifest <sortie.xlsx>`, avec `-root`
  obligatoire et `-out` disponible pour sélectionner l'index existant.
- L'export lit uniquement les shards référencés par `index.json` : il ne
  rescane pas les PDF et ne nécessite ni Poppler ni configuration d'extraction.
- Le modèle XLSX est embarqué dans l'exécutable avec ses trois onglets :
  `Manifest Template`, `Manifest Description` et `Manifest Sample`.
  Les deux premiers conservent leurs contenus et leur présentation ; seul
  `Manifest Sample` reçoit les données.
- Six colonnes, dans cet ordre : `DOS File`, `File Name`, `Batch Number`,
  `Work Order`, `Product Code`, `Batch Size`.
- `DOS File` commence à `VAL` inclus, sans lecteur ni répertoire parent ;
  `File Name` contient uniquement le nom et l'extension du PDF. Un document
  hors d'un dossier `VAL` provoque une erreur explicite.
- Une ligne par PDF physique, sans fusion de dossiers partageant un WO et
  un lot. Les documents en erreur d'extraction restent inclus et signalés ;
  les métadonnées absentes restent vides.
- Les valeurs sont écrites en texte, notamment pour préserver les zéros
  initiaux et ne pas interpréter un nom de fichier comme une formule.
- Validation des chemins, des doublons de chemin et de la longueur des
  cellules. Le fichier cible doit être nouveau et porter l'extension `.xlsx`.
  L'écriture passe par un fichier temporaire avant renommage.
- `-manifest` est incompatible avec `-test`, `-only` et `-force`.

### Corrections de reconnaissance

- Prise en charge des variantes des Batch Reports et rapports intermédiaires :
  `OF`, `WO`, `Work Order`, `Numéro de WO`, `Product code`, `Code article`,
  `Quantité du WO` et libellés français/anglais de numéro de lot.
- Nouvelle option `primary_only`, activée dans les configurations fournies :
  sélection du contexte principal avant les regex, puis une seule valeur
  conservée par champ, même si `max_matches` est supérieur à 1.
- Priorité à l'en-tête de la première page. Une valeur de lot doit suivre
  un libellé explicite sur la même ligne ; ni identifiant isolé, ni nom de
  fichier, ni valeur à la ligne suivante ne sert de remplacement.
- Les tableaux complémentaires ne sont retenus que lorsque leur WO
  correspond au WO principal. Les variantes avec préfixe `WO` et suffixe
  de présentation sont comparées sans réécrire l'identifiant conservé.
- Exception encadrée pour un ancien rapport de production dont l'identité
  d'en-tête est entièrement vide : le premier tableau peut la fournir
  s'il contient un WO non technique, un lot et un code produit explicites.
  Un en-tête partiel n'adopte pas l'identité d'une opération fille.
- Exclusion des lots consommés, références de préparation/stockage,
  WO SIP associés et tailles d'opérations filles.
- `Quantité transférée` peut fournir la taille manquante si un tableau du
  WO principal est identifié ou si le WO principal commence par `STO`.
  Les décimales, séparateurs de milliers et unités sont préservés.
- Les dates de fabrication restent fondées uniquement sur leurs libellés
  explicites. Aucune date d'opération, d'impression ou de transfert n'est
  transformée en date de fabrication.
- Prise en charge des fins de ligne LF/CRLF sans autoriser une capture
  de valeur sur la ligne suivante.

### Impacts sur le code et l'exploitation

- Configuration fournie : `max_pages` passe de 3 à 2, `max_matches` de 10 à 1,
  et `primary_only: true` est ajouté. Les deux pages sont extraites en un
  seul appel Poppler ; le filtrage travaille sur ce texte en mémoire.
- Les regex principales sont précompilées ; la recherche des champs s'arrête
  au premier match en mode principal. Aucun OCR ni appel PDF supplémentaire.
- `-workers` existait déjà. Les vérifications locales utilisent 16 workers ;
  le défaut CLI reste le nombre de processeurs, pas une constante 16.
- La version interne de l'extracteur passe de 3 à 4 et entre dans l'empreinte
  de configuration : le premier passage après mise à jour réextrait les
  documents des sections traitées. Les passages suivants restent incrémentaux.
- Le mode générique multivaleur reste disponible avec `primary_only: false`
  et une configuration adaptée. Cela ne restaure pas automatiquement les
  anciennes regex ni les anciennes limites de pages et de valeurs.
- Ajout de `github.com/xuri/excelize/v2` v2.9.1, de ses dépendances et mise
  à jour de `golang.org/x/text` vers v0.25.0. L'export est implémenté dans
  le paquet `manifest`, également importé par le finder : les dépendances
  XLSX augmentent donc la taille des deux exécutables, même si seul
  l'indexeur propose l'export.
- Les deux exécutables de distribution sont reconstruits avec les dépendances
  courantes. Aucun changement de code ni de comportement de l'interface finder
  n'est introduit ; `cmd/finder/ui.go` reste inchangé.
- Aucun PDF n'est modifié, renommé, déplacé ou supprimé. Ni déduplication,
  ni regroupement automatique des éditions, ni OCR n'est ajouté.

Tailles des binaires suivis par Git, avant/après reconstruction locale :

| Exécutable | Avant (octets) | Après (octets) |
|---|---:|---:|
| `dist/indexer.exe` | 3 092 480 | 6 734 336 |
| `dist/finder.exe` | 3 826 688 | 6 166 016 |

### Inventaire des fichiers du commit

Fichiers créés :

| Fichier | Rôle |
|---|---|
| `CHANGELOG.md` | Historique, justification de version, impacts et validation. |
| `internal/config/primary.go` | Sélection de l'identité principale et compléments contrôlés. |
| `internal/config/primary_test.go` | Régressions sur en-têtes, opérations filles, alignement, CRLF et mode générique. |
| `internal/manifest/xlsx.go` | Export XLSX depuis l'index, modèle embarqué et contrôle des chemins. |
| `internal/manifest/manifest_template.xlsx` | Modèle original à trois onglets embarqué dans l'exécutable. |

Fichiers modifiés :

| Fichier | Rôle du changement |
|---|---|
| `README.md`, `README.en.md` | Documentation cohérente FR/EN, nouvelle commande, configuration, migration et version proposée. |
| `cmd/indexer/main.go` | Branchement CLI d'export avant chargement de la configuration et de Poppler. |
| `internal/config/config.go` | Option `primary_only`, première correspondance et version d'extracteur 4. |
| `internal/config/config_test.go` | Formats Batch Report, tailles décimales et attentes de métadonnées principales. |
| `internal/manifest/manifest_test.go` | Contenu XLSX, conservation du modèle, chemins, texte et erreurs d'export. |
| `fields.json`, `dist/fields.json` | Alias de champs, captures alignées, deux pages et valeur principale unique. |
| `go.mod`, `go.sum` | Dépendance Excelize et dépendances transitives résolues. |
| `dist/indexer.exe`, `dist/finder.exe` | Exécutables Windows reconstruits. |

**Aucun fichier supprimé.** Les PDF, l'index local, sa configuration de
déploiement et les manifests générés ne font pas partie du commit.

### Validation locale du 17 novembre 2026

Résultats sur l'archive de travail ; ce ne sont ni un jeu de données distribué,
ni une garantie de performance sur un autre poste :

- `go test ./...` et `go vet ./...` réussissent.
- Réindexation complète de **1 798 PDF**, avec **16 workers** et **2 pages
  maximum**, en **57 secondes** ; un PDF de test de filtre reste sans champ
  reconnu et figure dans `errors.txt`.
- Aucun champ de l'index final ne contient plusieurs valeurs.

| Champ | PDF avec une valeur |
|---|---:|
| Ordre de fabrication | 1 797 |
| Numéro de lot | 1 787 |
| Code produit | 1 787 |
| Taille du lot | 1 152 |
| Début de fabrication | 706 |
| Fin de fabrication | 706 |

Par rapport à l'index intermédiaire multivaleur, et non à une mesure du tag
v1.0.1, le filtrage conserve la couverture WO, code produit et dates.
Quatre lots uniquement associés et 35 tailles d'opérations filles sont retirés
intentionnellement ; les champs correspondants restent vides plutôt que déduits.

Le classeur final a été comparé à l'index : **1 798 lignes de données**,
six colonnes dans l'ordre attendu, chemins `VAL\PDF\...` et noms sans chemin.
Les trois onglets sont présents ; valeurs, styles, formules, largeurs et
fusions des deux onglets de référence ont été contrôlés.
Un passage incrémental ultérieur retrouve **1 798 documents et 0 réextraction**.

### Déploiement et limites

Suivre la [migration du README](README.md#migration-depuis-v101) : déployer les
binaires et la configuration ensemble, réindexer toutes les sections puis
recharger le finder avant l'export.

Les champs peuvent rester vides si le libellé est absent, l'alignement
incompatible, la donnée secondaire ou au-delà des pages lues, ou le PDF sans
couche texte. Le PDF reste la référence. Les dates ne font pas partie des six
colonnes XLSX demandées. Le manifest n'est pas une validation réglementaire ;
les restrictions spécifiques de l'équipe destinataire restent à confirmer.
