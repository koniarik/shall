# The `shall serve` API

`shall serve` answers in JSON over HTTP, and its page builds the document
from the same answers, so a script or an agent reads a set exactly as a
reader sees it: ids resolved, statuses worked out, findings placed. The API
only reads; nothing it does changes a file.

The server listens on `127.0.0.1:7180` unless `-addr` gives another address.
While it listens on this machine only, it answers only requests that name
the host `localhost` or a loopback address.

The examples below are the answers for the example set at the end of
[format.md](format.md), served from the directory that holds its
`shall.yaml`.

## Conventions

- Every answer is `application/json` and carries `version`, the version of
  the files it was built from. The version changes whenever a file of a set
  changes, and differs from one start of the server to the next.
- A set, file or item that does not exist is answered with status 404 and
  `{"error": "…"}`, a sentence saying what is missing.
- Fields keep the names the format gives them. What the server works out is
  named in camelCase: `effectiveStatus`, `namedBy`, `itemId`, `retiredWith`.
- Paths are relative to the directory the server serves.

| Resource | Answer |
|---|---|
| `GET /api/sets` | The sets served, with their error and warning counts and their requirements by status |
| `GET /api/sets/{set}` | One set: its findings, its mission file with its items, and its chapters without theirs |
| `GET /api/sets/{set}/files/{path}` | One file with its items |
| `GET /api/sets/{set}/items/{id}` | One item with its children |
| `GET /api/sets/{set}/search?q={words}` | The items an id or words name |
| `GET /api/events` | A stream of versions, one each time the files change |

## References

Every id an item names comes as a reference, which says what the id names:

| `state` | Meaning | Also carries |
|---|---|---|
| `item` | An item of a set the server serves | `set`, `title`, `file` |
| `retired` | A retired id | `set`, `title` (the one it was retired under), `retiredWith` when an ancestor took it |
| `unknown` | Nothing the server serves | — |
| `mission` | The word `mission` in a need's `serves` | — |

`id` is the id as written. Where it names an item of another set, written
`name/id`, `set` is that set and `itemId` the item's id in it.

## Items

An item carries `id`, `kind` (`objective`, `stakeholder`, `need`,
`requirement` or `topic`), `title`, `text`, and the `file` and `line` it is
written at, then the fields of its kind as written: `measure`,
`stakeholders`, `serves`, `rationale`, `verify`, `status`, `notes`, `links`,
`mirrors`, `retired`, with every id as a reference. Fields an item leaves out
are left out. A topic's `text` is its Markdown as written.

What the server works out:

| Field | Meaning |
|---|---|
| `effectiveStatus` | A requirement's lowest status, counting the requirements below it, as [format.md](format.md) defines it |
| `namedBy` | The items that name or cite this one, each a reference with the `field` that names or cites it |
| `cites` | Its prose fields that cite an id, `text`, `measure`, `rationale` and `notes` (one entry per note), and on the mission file `mission`, each as runs: `text` runs, and `ref` runs that carry a `ref` |
| `findings` | The findings of `shall check` that concern this item |
| `children` | The item's children, of every kind, in full |
| `requirements` | A topic's requirements, all below it, by effective status |
| `blocks` | A topic's text read as Markdown, as below |

A topic's `blocks` are its text as blocks of text, each with a `type`:

| `type` | Carries |
|---|---|
| `heading` | `level`, from 1, and `inline` |
| `paragraph` | `inline` |
| `list` | `ordered`, `start` when ordered, and `items`, each a list of blocks |
| `quote` | `blocks` |
| `code` | `language` as its fence names it, and `text` |
| `diagram` | `language`, `mermaid` or `dot`, as the code block is marked, and `text`, its source; a `dot` diagram also carries `svg`, the server's drawing with only its shapes, text and styles, or `error`, why Graphviz cannot draw it |
| `table` | `align` per column (`left`, `center`, `right` or none), `head`, a list of cells, and `rows`, each a list of cells; a cell is a list of `inline` |
| `rule` | nothing more |
| `html` | `text`, the HTML as written, to be shown as text |

Within a block, `inline` is a list of runs, each with a `type`: `text` and
`code` carry `text`; `emphasis` and `strong` carry `inline`; `link` carries
`url`, `title` when it has one, and `inline`, an image coming as a link to
it; `break` carries nothing; `ref`, an id the text cites, carries `ref`, a
reference. HTML within a line comes as `text`.

## `GET /api/sets`

```json
{
  "version": "dm7japskbab4.1",
  "sets": [
    {
      "name": "coffee",
      "manifest": "shall.yaml",
      "errors": 0,
      "warnings": 0,
      "requirements": {
        "total": 2,
        "draft": 0,
        "approved": 0,
        "implemented": 2,
        "verified": 0,
        "waived": 0,
        "other": 0
      }
    }
  ],
  "clashes": [],
  "findings": []
}
```

`requirements` counts each set's requirements, children included, by
effective status; `other` counts those with no status or one the format
does not define. `clashes` lists names that several sets share, each with
their manifests; neither set is served under it. `findings` are those that belong to no set
served under its name.

## `GET /api/sets/{set}`

The set's files in the order they read: the mission file first, with its
items, then the chapters in path order, each with its title, counts and the
findings that concern the file itself, but not its items. `requirements`
counts the set's requirements by effective status, as each file's `counts`
does for the file. `findings` at the
top are those that concern no file of the set, such as the manifest's. A file
that cannot be read or parsed has `"broken": true` and its findings.
`depends` gives, for each name the manifest's `depends` gives a set the
server serves, that set's own name, such as `{"company": "acme-standards"}`;
it is left out when there is none.

```json
{
  "version": "dm7japskbab4.1",
  "name": "coffee",
  "manifest": "shall.yaml",
  "requirements": {
    "total": 2,
    "draft": 0,
    "approved": 0,
    "implemented": 2,
    "verified": 0,
    "waived": 0,
    "other": 0
  },
  "findings": [],
  "files": [
    {
      "path": "requirements/mission.shall.yaml",
      "kind": "mission",
      "title": "Why The Machine Exists",
      "counts": {
        "items": 3,
        "findings": 0,
        "requirements": {
          "total": 0,
          "draft": 0,
          "approved": 0,
          "implemented": 0,
          "verified": 0,
          "waived": 0,
          "other": 0
        }
      },
      "findings": [],
      "mission": "Coffee in the office without anyone looking after the machine.",
      "ranking": [
        {
          "id": "O1",
          "state": "item",
          "set": "coffee",
          "title": "No Dry Brews",
          "file": "requirements/mission.shall.yaml"
        }
      ],
      "items": [
        {
          "id": "O1",
          "kind": "objective",
          "title": "No Dry Brews",
          "text": "No brew starts while the water tank is empty.",
          "file": "requirements/mission.shall.yaml",
          "line": 5,
          "measure": "brews started on an empty tank, per thousand brews.",
          "namedBy": [
            {
              "field": "serves",
              "id": "N2.1",
              "state": "item",
              "set": "coffee",
              "title": "Know Before Brewing",
              "file": "requirements/mission.shall.yaml"
            }
          ]
        },
        {
          "id": "S2",
          "kind": "stakeholder",
          "title": "Office User",
          "text": "Wants coffee without having to look after the machine.",
          "file": "requirements/mission.shall.yaml",
          "line": 9,
          "children": [
            {
              "id": "N2.1",
              "kind": "need",
              "title": "Know Before Brewing",
              "text": "Know before I brew that the tank needs water.",
              "file": "requirements/mission.shall.yaml",
              "line": 13,
              "status": "validated",
              "serves": [
                {
                  "id": "O1",
                  "state": "item",
                  "set": "coffee",
                  "title": "No Dry Brews",
                  "file": "requirements/mission.shall.yaml"
                }
              ],
              "notes": [
                "Heard from three users in the first week of the trial."
              ],
              "namedBy": [
                {
                  "field": "serves",
                  "id": "R3.1",
                  "state": "item",
                  "set": "coffee",
                  "title": "Refill Lamp",
                  "file": "requirements/01-tank.shall.yaml"
                }
              ]
            }
          ]
        }
      ]
    },
    {
      "path": "requirements/01-tank.shall.yaml",
      "kind": "chapter",
      "title": "Water Tank",
      "counts": {
        "items": 3,
        "findings": 0,
        "requirements": {
          "total": 2,
          "draft": 0,
          "approved": 0,
          "implemented": 2,
          "verified": 0,
          "waived": 0,
          "other": 0
        }
      },
      "findings": [],
      "retired": [
        {
          "id": "R4",
          "title": "Water Handling",
          "file": "requirements/01-tank.shall.yaml",
          "line": 27
        }
      ]
    }
  ]
}
```

## `GET /api/sets/{set}/files/{path}`

One file with its items in reading order, children nested.

```json
{
  "version": "dm7japskbab4.1",
  "path": "requirements/01-tank.shall.yaml",
  "kind": "chapter",
  "title": "Water Tank",
  "counts": {
    "items": 3,
    "findings": 0,
    "requirements": {
      "total": 2,
      "draft": 0,
      "approved": 0,
      "implemented": 2,
      "verified": 0,
      "waived": 0,
      "other": 0
    }
  },
  "findings": [],
  "items": [
    {
      "id": "T3",
      "kind": "topic",
      "title": "Water Level",
      "text": "The tank is *empty* once the water is below the pump's intake.\n",
      "file": "requirements/01-tank.shall.yaml",
      "line": 3,
      "children": [
        {
          "id": "R3.1",
          "kind": "requirement",
          "title": "Refill Lamp",
          "text": "When the tank becomes empty, the machine SHALL light the refill lamp.",
          "file": "requirements/01-tank.shall.yaml",
          "line": 8,
          "status": "implemented",
          "effectiveStatus": "implemented",
          "serves": [
            {
              "id": "N2.1",
              "state": "item",
              "set": "coffee",
              "title": "Know Before Brewing",
              "file": "requirements/mission.shall.yaml"
            }
          ],
          "links": [
            {
              "url": "https://ci.example.com/runs/4711",
              "title": "Tank tests on hardware revision C"
            }
          ],
          "retired": [
            {
              "id": "R3.1.2",
              "title": "Refill Beep",
              "file": "requirements/01-tank.shall.yaml",
              "line": 24
            }
          ],
          "children": [
            {
              "id": "R3.1.1",
              "kind": "requirement",
              "title": "Lamp Seen From The Door",
              "text": "The machine SHALL show the refill lamp to anyone standing at the kitchen door.",
              "file": "requirements/01-tank.shall.yaml",
              "line": 17,
              "status": "implemented",
              "effectiveStatus": "implemented",
              "verify": "inspection"
            }
          ]
        }
      ],
      "requirements": {
        "total": 2,
        "draft": 0,
        "approved": 0,
        "implemented": 2,
        "verified": 0,
        "waived": 0,
        "other": 0
      },
      "blocks": [
        {
          "type": "paragraph",
          "inline": [
            {
              "type": "text",
              "text": "The tank is "
            },
            {
              "type": "emphasis",
              "inline": [
                {
                  "type": "text",
                  "text": "empty"
                }
              ]
            },
            {
              "type": "text",
              "text": " once the water is below the pump's intake."
            }
          ]
        }
      ]
    }
  ],
  "retired": [
    {
      "id": "R4",
      "title": "Water Handling",
      "file": "requirements/01-tank.shall.yaml",
      "line": 27
    }
  ]
}
```

## `GET /api/sets/{set}/items/{id}`

```json
{
  "version": "dm7japskbab4.1",
  "id": "R3.1.1",
  "kind": "requirement",
  "title": "Lamp Seen From The Door",
  "text": "The machine SHALL show the refill lamp to anyone standing at the kitchen door.",
  "file": "requirements/01-tank.shall.yaml",
  "line": 17,
  "status": "implemented",
  "effectiveStatus": "implemented",
  "verify": "inspection"
}
```

A retired id is answered with 404, the error, and the entry that retired it:

```json
{
  "error": "R4 was retired at requirements/01-tank.shall.yaml:27.",
  "retired": {
    "id": "R4",
    "title": "Water Handling",
    "file": "requirements/01-tank.shall.yaml",
    "line": 27
  }
}
```

## `GET /api/sets/{set}/search?q={words}`

What a reader typed names. A single word that names an item as an id comes
first with `"match": "id"`: an id of the set, or `name/id` of another, where
`name` is a name the manifest gives under `depends` or the name of a set the
server serves, matched exactly first and then in capitals. Then come the
items of the set whose id or title holds every word, with `"match":
"title"`, and those whose text does, with `"match": "text"`, each group in
reading order, ignoring case. At most 50 are listed, and `more` counts the
rest.

When nothing is found, `message` says why: that no item holds the words, or,
for a word written as an id or as `name/id` and for a retired id, what the
id names.

```json
{
  "version": "dm7japskbab4.1",
  "query": "lamp",
  "results": [
    {
      "match": "title",
      "id": "R3.1",
      "state": "item",
      "set": "coffee",
      "title": "Refill Lamp",
      "file": "requirements/01-tank.shall.yaml"
    },
    {
      "match": "title",
      "id": "R3.1.1",
      "state": "item",
      "set": "coffee",
      "title": "Lamp Seen From The Door",
      "file": "requirements/01-tank.shall.yaml"
    }
  ]
}
```

```json
{
  "version": "dm7japskbab4.1",
  "query": "R4",
  "results": [],
  "message": "R4 was retired at requirements/01-tank.shall.yaml:27."
}
```

## `GET /api/events`

A stream of [server-sent events](https://html.spec.whatwg.org/multipage/server-sent-events.html).
The server sends the current version as soon as the stream opens, and the new
one each time the files change, within about two seconds of a save:

```text
event: version
data: dm7japskbab4.1
```

A client that sees a version other than the one its answers carry fetches
them again.

## A static copy

`shall export -out site [-base path] [directory]` writes into `site`, which
must be new or empty, every answer above that stays the same while the files
do, each as a file named by its address with `.json` added:

- `api/sets.json`;
- `api/sets/{set}.json`;
- `api/sets/{set}/files/{path}.json`;
- `api/sets/{set}/items/{id}.json`, also for each retired id, holding the
  404 answer the server would send.

A set's folders are named by its name as it is; a name that cannot name a
folder, such as one holding `/`, fails the export. An id has a file when it
names an item or is retired itself. The server also answers an id retired
only with an ancestor, or written with another letter at a retired path;
in a copy such an id has no file, and the page says no item has it.

Their `version` is `copy`. Search and events have no files: the copy's page
searches the copy's files as the server would, reading `depends` in the set's
answer for the names it gives other sets, and nothing changes while it is
open.

The page is at every address a reader can open: `index.html` for the start,
`{set}/index.html` for each set and `{set}/{id}.html` for each item. Any
other address is answered by `404.html`, which a host such as GitHub Pages
serves for addresses it lacks. `-base` is the path the host serves the copy
under, `/` by default; GitHub Pages serves a project's site under
`/{repository}/`. A static host sends no headers of the copy's choosing, so
the page carries its policy in a meta tag, which cannot keep it out of
frames.
