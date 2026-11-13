# EBM Archive

[English version](README.en.md)

EBM Archive est un outil Windows pour retrouver rapidement des PDF dans une archive volumineuse, notamment sur un partage réseau. Il extrait quelques métadonnées configurables des PDF, les indexe, puis permet de rechercher un document et de l’ouvrir depuis une interface en terminal.

L’outil ne modifie pas les PDF et ne remplace pas l’archive documentaire : il construit un index de recherche à côté des fichiers. Le PDF ouvert reste le document de référence.

<img width="1933" height="1027" alt="image" src="https://github.com/user-attachments/assets/36c66b78-0d78-437e-9f9f-94bd4443d08c" />


## Les deux exécutables

| Exécutable | Rôle | Prérequis |
|---|---|---|
| `indexer.exe` | Parcourt l’archive et extrait les champs des PDF pour créer ou actualiser l’index. Il est destiné à une machine d’indexation, par exemple via une tâche planifiée. | Go pour la compilation ; Poppler (`pdftotext.exe`) à l’exécution. |
| `finder.exe` | Recherche dans l’index et ouvre le PDF sélectionné ou son dossier dans l’Explorateur. | Aucun logiciel Go ou Poppler à installer sur le poste utilisateur. |

## Fonctionnement

1. L’indexeur repère les PDF placés directement à la racine ainsi que ceux des sous-dossiers de premier niveau, qu’il parcourt récursivement.
2. `pdftotext` extrait le texte des premières pages (3 par défaut).
3. Les expressions régulières de `fields.json` repèrent les champs à conserver, par exemple le numéro de lot. Les PDF sont lus ; leur contenu n’est pas changé.
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

La compilation fournie cible Windows 64 bits. Elle nécessite Go 1.24 ou ultérieur.

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

Poussez un tag de version (`v1.0.0`, par exemple) pour lancer la compilation et publier les assets dans une GitHub Release :

```powershell
git tag v1.0.0
git push origin v1.0.0
```

Le workflow crée la Release si elle n’existe pas et y ajoute `finder.exe`, `indexer.exe` et `fields.json`. Si la Release existe déjà pour ce tag, les assets portant ces noms sont remplacés.

## Indexeur

Installez Poppler pour Windows et repérez le chemin vers `pdftotext.exe`. Depuis PowerShell :

```powershell
# Vérifier l’extraction et les expressions régulières sur un PDF
.\dist\indexer.exe -test "\\SERVEUR\EBM\VAL\exemple.pdf" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe"

# Indexer ou actualiser l’archive
.\dist\indexer.exe -root "\\SERVEUR\EBM" `
  -pdftotext "C:\poppler\Library\bin\pdftotext.exe" -workers 8
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

L’indexation est incrémentale : un PDF est retraité si sa taille, sa date de modification ou la configuration d’extraction a changé. Les erreurs d’extraction susceptibles d’être temporaires sont retentées au passage suivant. L’indexeur empêche deux exécutions simultanées sur le même dossier d’index et écrit les manifestes de façon atomique.

`errors.txt` recense les PDF en erreur ou sans champ détecté. Un PDF scanné sans couche texte ne peut pas fournir de métadonnées sans OCR ; l’OCR n’est pas effectué par cet outil.

## Configuration des champs

`fields.json` définit le nombre de pages à lire, les dossiers à exclure et les champs à extraire :

```json
{
  "max_pages": 3,
  "max_matches": 10,
  "exclude_dirs": ["$RECYCLE.BIN", "System Volume Information"],
  "fields": [
    {
      "name": "lot",
      "label": "N° de lot",
      "regex": "(?i:N°|No\\.?)\\s*(?i:de\\s+)?(?i:lot)\\s*[:.]?\\s*([A-Z0-9/-]*[0-9][A-Z0-9/-]*)",
      "uppercase": true
    }
  ]
}
```

- Les expressions régulières suivent la syntaxe RE2 de Go : les lookaheads ne sont pas pris en charge.
- Le premier groupe capturé est la valeur enregistrée ; sans groupe, le match complet est utilisé.
- `uppercase: true` normalise les valeurs en majuscules.
- `sections` limite un champ aux sections dont le chemin contient l’un des segments indiqués. Sans `sections`, le champ s’applique à toutes les sections.
- Modifier la configuration entraîne la réextraction des documents concernés lors de l’indexation suivante.

La configuration fournie contient le numéro de lot pour toutes les sections et des champs dédiés à `VAL` : ordre de fabrication, code produit, dates de début et de fin de fabrication et taille du lot.

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
