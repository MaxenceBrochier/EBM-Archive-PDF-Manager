# EBM Archive

[English version](README.en.md)

**Prochaine version proposée : v1.1.0**, après v1.0.1. Voir le [journal des changements](CHANGELOG.md) pour les nouveautés, les impacts et les résultats de validation.

EBM Archive est un outil Windows pour retrouver rapidement des PDF dans une archive volumineuse, notamment sur un partage réseau. Il extrait quelques métadonnées configurables des PDF, les indexe, puis permet de rechercher un document et de l’ouvrir depuis une interface en terminal.

L’outil ne modifie pas les PDF et ne remplace pas l’archive documentaire : il construit un index de recherche à côté des fichiers. Le PDF ouvert reste le document de référence.

## Les deux exécutables

| Exécutable | Rôle | Prérequis |
|---|---|---|
| `indexer.exe` | Parcourt l’archive et extrait les champs des PDF pour créer ou actualiser l’index ; exporte aussi l’index en manifest XLSX. | Go pour la compilation ; Poppler (`pdftotext.exe`) pour l’extraction PDF uniquement. |
| `finder.exe` | Recherche dans l’index et ouvre le PDF sélectionné ou son dossier dans l’Explorateur. | Aucun logiciel Go ou Poppler à installer sur le poste utilisateur. |

## Fonctionnement

1. L’indexeur repère les PDF placés directement à la racine ainsi que ceux des sous-dossiers de premier niveau, qu’il parcourt récursivement.
2. `pdftotext` extrait le texte des premières pages (2 avec la configuration fournie).
3. La configuration fournie sélectionne les métadonnées principales de l’en-tête, puis applique les expressions régulières de `fields.json`, avec une seule valeur par champ. Les PDF sont lus ; leur contenu n’est pas changé.
4. L’indexeur écrit un index dans `.index`, avec un fichier compressé par section.
5. Le finder charge cet index et permet de filtrer les résultats avant d’ouvrir le document voulu.

Les dossiers cachés et ceux déclarés dans `exclude_dirs` sont ignorés. Les PDF à la racine forment la section « Racine » ; chaque sous-dossier de premier niveau forme une section (un onglet). L’onglet « Tous » regroupe les sections.

```text
<archive>\
├── finder.exe
├── fields.json
├── .index\
│   ├── index.json
│   ├── _racine.json.gz
│   ├── <section>.json.gz
│   └── errors.txt
├── VAL\
│   └── ...
└── <autres sections>\
```

Le nom du shard de la racine est `_racine.json.gz` ; les autres shards sont nommés d’après leur section.

## Compilation

La compilation fournie cible Windows 64 bits. Le module nécessite Go 1.24.2 ou ultérieur et déclare la toolchain Go 1.24.7, également utilisée en CI.

```powershell
.\build.ps1
```

Le script lance `go mod tidy`, `go vet ./...` et `go test ./...`, puis construit les exécutables dans `dist\` et y copie `fields.json`.

## Tests et intégration continue

Le [workflow GitHub Actions](.github/workflows/go.yml) lance `go vet`, les tests avec couverture et la compilation sur Windows à chaque push ou pull request vers `main`. Il peut aussi être lancé manuellement depuis GitHub. Pour lancer ces vérifications localement :

```powershell
go vet ./...
go test ./... -cover
go build ./...
```

## Releases

La version proposée est `v1.1.0` : l’export XLSX justifie une version mineure, accompagnée de corrections d’extraction. Le nombre de corrections ne détermine pas les numéros : `MAJEUR.MINEUR.PATCH` augmente selon le niveau de changement, et les niveaux suivants repartent à zéro. Le format de l’index et les commandes existantes sont conservés ; la sélection des métadonnées fournie change volontairement, comme décrit ci-dessous.

Après validation de la version et du commit à publier, poussez son tag pour lancer la compilation et publier les assets dans une GitHub Release :

```powershell
git tag v1.1.0
git push origin v1.1.0
```

Le workflow crée la Release si elle n’existe pas et y ajoute `finder.exe`, `indexer.exe` et `fields.json`. Si la Release existe déjà pour ce tag, les assets portant ces noms sont remplacés.

### Migration depuis v1.0.1

1. Déployez les exécutables reconstruits et le nouveau `fields.json`. Si votre tâche utilise `-config`, mettez aussi à jour cette copie de configuration.
2. Relancez l’indexation. La version interne de l’extracteur passe de 3 à 4 et invalide le cache : le premier passage réextrait les documents des sections traitées, sans nécessiter `-force`. Pour mettre toute l’archive à jour, n’utilisez pas `-only`.
3. Rechargez l’index dans le finder (`F5`), puis exportez le manifest si nécessaire.

```powershell
.\dist\indexer.exe -root "\\SERVEUR\EBM\VAL\PDF" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe" -workers 16
.\dist\indexer.exe -root "\\SERVEUR\EBM\VAL\PDF" `
  -manifest ".\manifest-v1.1.0.xlsx"
```

La configuration fournie passe de 3 à 2 pages et de 10 valeurs possibles à une valeur principale par champ. Les références secondaires ne sont donc plus recherchables par ces champs et les données situées après la page 2 ne sont pas lues. L’absence de valeur fiable laisse une cellule vide ; elle ne prouve pas l’absence de cette donnée ailleurs dans le PDF. Pour une configuration générique personnalisée, `primary_only: false`, `max_matches` et `max_pages` restent disponibles ; désactiver `primary_only` ne restaure pas à lui seul les anciennes expressions régulières.

## Indexeur

Installez Poppler pour Windows et repérez le chemin vers `pdftotext.exe`. Depuis PowerShell :

```powershell
# Vérifier l’extraction et les expressions régulières sur un PDF
.\dist\indexer.exe -test "\\SERVEUR\EBM\VAL\exemple.pdf" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe"

# Indexer ou actualiser l’archive
.\dist\indexer.exe -root "\\SERVEUR\EBM" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe" -workers 16
```

Options principales :

| Option | Valeur par défaut | Description |
|---|---|---|
| `-root` | Aucune | Dossier racine de l’archive ; obligatoire hors mode `-test`. |
| `-out` | `<root>\.index` | Dossier de sortie de l’index. |
| `-config` | `fields.json` à côté de l’exécutable (sinon dans le chemin de recherche courant) | Fichier de définition des champs. |
| `-pdftotext` | `pdftotext` | Chemin vers Poppler `pdftotext.exe` si la commande n’est pas dans le `PATH`. |
| `-workers` | Nombre de processeurs | Nombre d’extractions en parallèle. |
| `-timeout` | `60s` | Durée maximale d’extraction d’un PDF. |
| `-only` | Toutes les sections | Limite le traitement à une ou plusieurs sections séparées par des virgules. |
| `-force` | Désactivé | Réextrait tous les PDF des sections traitées. |
| `-test` | — | Affiche le texte extrait et les valeurs détectées pour un PDF, puis quitte. |
| `-manifest` | — | Exporte l'index existant vers le fichier `.xlsx` indiqué, puis quitte sans réindexer. |

L’indexation est incrémentale : un PDF est retraité si sa taille, sa date de modification, la configuration ou la version interne de l’extracteur a changé. Les erreurs d’extraction susceptibles d’être temporaires sont retentées au passage suivant. L’indexeur empêche deux exécutions simultanées sur le même dossier d’index et écrit les fichiers d’index de façon atomique.

`errors.txt` recense les PDF en erreur ou sans champ détecté. Un PDF scanné sans couche texte ne peut pas fournir de métadonnées sans OCR ; l’OCR n’est pas effectué par cet outil.

### Export du manifest XLSX

```powershell
.\dist\indexer.exe -root "\\SERVEUR\EBM\VAL\PDF" -manifest ".\manifest.xlsx"
```

Le mode `-manifest` lit l'index existant dans `<root>\.index` (ou dans le dossier passé à `-out`). Il ne nécessite ni Poppler ni `fields.json`. Actualisez d'abord l'index si les PDF ou leurs métadonnées ont changé. Le fichier de sortie ne doit pas déjà exister. `-manifest` ne se combine pas avec `-test`, `-only` ou `-force`.

Le classeur reprend les trois onglets et la mise en forme du modèle XLSX fourni, embarqué dans l'exécutable : `Manifest Template`, `Manifest Description` et `Manifest Sample`. Seul `Manifest Sample` est rempli avec une ligne par PDF et les six colonnes suivantes, dans cet ordre. `Manifest Template` et `Manifest Description` sont conservés sans modification.

| Colonne | Contenu |
|---|---|
| `DOS File` | Chemin du PDF commençant à `VAL` inclus, par exemple `VAL\PDF\Batch Report\2015\rapport.pdf`. Aucune lettre de lecteur ni aucun dossier parent n'est exporté. |
| `File Name` | Nom du fichier PDF, extension comprise, sans le chemin. |
| `Batch Number` | Numéro de lot principal (`lot`). |
| `Work Order` | WO/OF principal (`of`). |
| `Product Code` | Code produit principal (`code_produit`), conservé en texte pour préserver les zéros initiaux. |
| `Batch Size` | Taille principale (`taille_lot`), ou quantité transférée admissible, avec son unité. |

Les colonnes conservent les styles du modèle : chemin et nom en premier, puis les quatre métadonnées personnalisées. Chaque PDF doit être situé sous un dossier nommé `VAL` (sans distinction de casse), sinon l'export échoue sans créer de classeur. La configuration fournie conserve une seule valeur principale par champ. Les métadonnées absentes restent vides ; les documents en erreur d'extraction sont inclus et signalés dans le terminal. Chaque édition PDF reste une ligne distincte, même si plusieurs fichiers partagent le même WO et lot.

Les colonnes techniques RIM/IT restent visibles dans l'onglet descriptif d'origine mais ne sont pas remplies avec des valeurs inventées. Les PDF ne sont pas renommés ni déplacés ; seuls les chemins exportés sont limités à `VAL\...`. Les restrictions de caractères mentionnées par le modèle restent à vérifier selon le document d'exigences de l'équipe destinataire.

## Configuration des champs

`fields.json` définit le nombre de pages à lire, les dossiers à exclure et les champs à extraire :

```json
{
  "max_pages": 2,
  "max_matches": 1,
  "primary_only": true,
  "exclude_dirs": ["$RECYCLE.BIN", "System Volume Information"],
  "fields": [
    {
      "name": "lot",
      "label": "N° de lot",
      "regex": "(?mi)^[ \\t]*(?:Lot Number|Batch Number|Numéro de lot|N° de lot|Lot)[ \\t]*:[ \\t]*([A-Z0-9/-]*[0-9][A-Z0-9/-]*)(?:[ \\t\\r]|$)",
      "uppercase": true
    }
  ]
}
```

- Les expressions régulières suivent la syntaxe RE2 de Go : les lookaheads ne sont pas pris en charge.
- Le premier groupe capturé est la valeur enregistrée ; sans groupe, le match complet est utilisé.
- `uppercase: true` normalise les valeurs en majuscules.
- `sections` limite un champ aux sections dont le chemin contient l’un des segments indiqués. Sans `sections`, le champ s’applique à toutes les sections.
- `primary_only: true` limite la recherche à l'en-tête de la première page et conserve un seul résultat par champ. Les tableaux de métadonnées complémentaires ne sont utilisés que si leur WO correspond au WO principal. Pour un ancien rapport de production dont l'identité d'en-tête est entièrement vide, le premier tableau peut fournir l'identité s'il contient un WO non technique, un lot et un code produit explicites. Les opérations filles, consommations et références SIP sont exclues. Sans cette option, l'extraction générique reste disponible avec la limite `max_matches`.
- Les libellés de lot reconnus incluent `Lot`, `Lot Number`, `Batch Number`, `Numéro de lot` et `N° de lot`. La valeur doit être à droite sur la même ligne : les retours à la ligne, identifiants isolés et noms de fichiers ne servent pas de valeur de remplacement.
- Modifier la configuration entraîne la réextraction des documents concernés lors de l’indexation suivante.

La configuration fournie contient le numéro de lot pour toutes les sections et des champs dédiés à `VAL` : ordre de fabrication, code produit, dates de début et de fin de fabrication et taille du lot.
Les sections `Batch Report` et `Batch Report intermédiaire` extraient aussi l'ordre de fabrication (`OF`, `WO`, `Numéro de WO`), le code produit (`Product code`, `Code article`) et la taille du lot (`Quantité du WO`). Les WO sont recherchables avec `of:WO3399985`. Un WO seul n'est pas utilisé comme numéro de lot si le PDF ne contient pas de champ lot.
La taille du lot peut utiliser la première `Quantité transférée`, avec ses décimales et son unité, si aucune taille principale n'est disponible et qu'un tableau du WO principal est identifié ou que le WO principal commence par `STO`. Ce complément peut provenir de la seconde page déjà extraite. Les références aux lots de préparation buffer ou de solution dans la cuve ne remplacent pas un lot principal absent. Le champ reste vide dans ce cas. Aucun OCR ni lecture de pages supplémentaires n'est ajouté.

Les dates de fabrication exigent les libellés explicites `Début de Fabrication` et `Fin de Fabrication`. Une date d’opération, de transfert ou d’impression n’est pas utilisée à leur place. Ces dates restent des champs de recherche dans l’index ; elles ne font pas partie des six colonnes du manifest demandé.

## Finder

Placez `finder.exe` à la racine de l’archive, à côté de `.index`, puis lancez-le. Sans option, il cherche l’index à côté de l’exécutable ; `-root` permet d’indiquer un autre chemin :

```powershell
.\finder.exe
.\finder.exe -root "\\SERVEUR\EBM"
```

La recherche est insensible à la casse. Plusieurs mots sont combinés en « ET ». On peut rechercher partout ou restreindre un terme à un champ avec son nom technique, par exemple :

```text
lot:L24A
VAL 17CU004
```

| Touche | Action |
|---|---|
| Saisie | Filtrer les résultats en temps réel. |
| `Tab` / `Maj+Tab` | Passer à l’onglet suivant / précédent. |
| `↑` / `↓`, `PgUp` / `PgDn` | Parcourir les résultats. |
| `Entrée` | Ouvrir le PDF avec l’application Windows associée. |
| `Ctrl+O` | Ouvrir l’Explorateur sur le PDF sélectionné. |
| `Ctrl+R` / `F5` | Recharger l’index. |
| `Échap` | Effacer la recherche, puis quitter si elle est déjà vide. |

Jusqu’à 1 000 résultats sont affichés à la fois ; le finder indique le nombre total de correspondances.

## Exécution planifiée et droits

Le fichier `scheduled-task.ps1` donne un exemple de tâche quotidienne à 2 h. Adaptez-y les chemins de l’exécutable, de l’archive, de Poppler et le compte de service avant utilisation.

Le compte qui exécute l’indexeur doit pouvoir lire les PDF et écrire dans `.index`. Les utilisateurs du finder ont besoin d’un accès en lecture aux PDF et à l’index. Sur un partage d’entreprise, vérifiez également les règles d’exécution des exécutables depuis le réseau avec l’équipe IT.

## Limites d’usage

Le finder est un outil de recherche et de navigation, pas un système de validation ou d’archivage. Les PDF sans texte exploitable ou sans champ détecté ne seront pas trouvables par ces champs ; consultez `errors.txt` pour les repérer.
