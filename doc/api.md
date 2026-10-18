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
| `GET /api/sets` | The sets served, with their error and warning counts |
| `GET /api/sets/{set}` | One set: its findings, its mission file with its items, and its chapters without theirs |
| `GET /api/sets/{set}/files/{path}` | One file with its items |
| `GET /api/sets/{set}/items/{id}` | One item with its children |
| `GET /api/sets/{set}/resolve?id={id}` | What an id names, as a reader would type it |
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

An item carries `id`, `kind` (`objective`, `stakeholder`, `need` or
`requirement`), `title`, `text`, and the `file` and `line` it is written at,
then the fields of its kind as written: `measure`, `stakeholder`, `serves`,
`rationale`, `verify`, `status`, `notes`, `links`, `mirrors`, `retired`, with
every id as a reference. Fields an item leaves out are left out.

What the server works out:

| Field | Meaning |
|---|---|
| `effectiveStatus` | A requirement's lowest status, counting its children, as [format.md](format.md) defines it |
| `namedBy` | The items that name this one, each a reference with the `field` that names it |
| `findings` | The findings of `shall check` that concern this item |
| `children` | The item's children, in full |

## `GET /api/sets`

```json
{
  "version": "dm7japskbab4.1",
  "sets": [
    {
      "name": "coffee",
      "manifest": "shall.yaml",
      "errors": 0,
      "warnings": 0
    }
  ],
  "clashes": [],
  "findings": []
}
```

`clashes` lists names that several sets share, each with their manifests;
neither set is served under it. `findings` are those that belong to no set
served under its name.

## `GET /api/sets/{set}`

The set's files in the order they read: the mission file first, with its
items, then the chapters in path order, each with its title, counts and the
findings that concern the file itself, but not its items. `findings` at the
top are those that concern no file of the set, such as the manifest's. A file
that cannot be read or parsed has `"broken": true` and its findings.

```json
{
  "version": "dm7japskbab4.1",
  "name": "coffee",
  "manifest": "shall.yaml",
  "findings": [],
  "files": [
    {
      "path": "requirements/mission.shall.yaml",
      "kind": "mission",
      "counts": {
        "items": 3,
        "findings": 0
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
      "objectives": [
        {
          "id": "O1",
          "kind": "objective",
          "title": "No Dry Brews",
          "text": "No brew starts while the water tank is empty.",
          "file": "requirements/mission.shall.yaml",
          "line": 4,
          "measure": "brews started on an empty tank, per thousand brews.",
          "namedBy": [
            {
              "field": "serves",
              "id": "N1",
              "state": "item",
              "set": "coffee",
              "title": "Know Before Brewing",
              "file": "requirements/mission.shall.yaml"
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
        "items": 2,
        "findings": 0
      },
      "findings": [],
      "retired": [
        {
          "id": "R2",
          "title": "Water Handling",
          "file": "requirements/01-tank.shall.yaml",
          "line": 22
        }
      ]
    }
  ]
}
```

The mission file's `stakeholders` and `needs` are left out of this example;
the server sends them after `objectives`, each item in the same shape.

## `GET /api/sets/{set}/files/{path}`

One file with its items, children nested; `requirements` for a chapter,
`objectives`, `stakeholders` and `needs` for the mission file.

```json
{
  "version": "dm7japskbab4.1",
  "path": "requirements/01-tank.shall.yaml",
  "kind": "chapter",
  "title": "Water Tank",
  "counts": {
    "items": 2,
    "findings": 0
  },
  "findings": [],
  "requirements": [
    {
      "id": "R1",
      "kind": "requirement",
      "title": "Refill Lamp",
      "text": "When the tank becomes empty, the machine SHALL light the refill lamp.",
      "file": "requirements/01-tank.shall.yaml",
      "line": 3,
      "status": "implemented",
      "effectiveStatus": "implemented",
      "serves": [
        {
          "id": "N1",
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
          "id": "R1.2",
          "title": "Refill Beep",
          "file": "requirements/01-tank.shall.yaml",
          "line": 19
        }
      ],
      "children": [
        {
          "id": "R1.1",
          "kind": "requirement",
          "title": "Lamp Seen From The Door",
          "text": "The machine SHALL show the refill lamp to anyone standing at the kitchen door.",
          "file": "requirements/01-tank.shall.yaml",
          "line": 12,
          "status": "implemented",
          "effectiveStatus": "implemented",
          "verify": "inspection"
        }
      ]
    }
  ],
  "retired": [
    {
      "id": "R2",
      "title": "Water Handling",
      "file": "requirements/01-tank.shall.yaml",
      "line": 22
    }
  ]
}
```

## `GET /api/sets/{set}/items/{id}`

```json
{
  "version": "dm7japskbab4.1",
  "id": "R1.1",
  "kind": "requirement",
  "title": "Lamp Seen From The Door",
  "text": "The machine SHALL show the refill lamp to anyone standing at the kitchen door.",
  "file": "requirements/01-tank.shall.yaml",
  "line": 12,
  "status": "implemented",
  "effectiveStatus": "implemented",
  "verify": "inspection"
}
```

A retired id is answered with 404, the error, and the entry that retired it:

```json
{
  "error": "R2 was retired at requirements/01-tank.shall.yaml:22.",
  "retired": {
    "id": "R2",
    "title": "Water Handling",
    "file": "requirements/01-tank.shall.yaml",
    "line": 22
  }
}
```

## `GET /api/sets/{set}/resolve?id={id}`

What an id names, as a reader would type it: an id of the set, or `name/id`
of another, where `name` is a name the manifest gives under `depends` or the
name of a set the server serves. Ids are matched exactly first, then in
capitals. The answer is always 200; a `message` says why an id names no item.

```json
{
  "version": "dm7japskbab4.1",
  "ref": {
    "id": "r1",
    "state": "item",
    "set": "coffee",
    "itemId": "R1",
    "title": "Refill Lamp",
    "file": "requirements/01-tank.shall.yaml"
  }
}
```

```json
{
  "version": "dm7japskbab4.1",
  "ref": {
    "id": "R2",
    "state": "retired",
    "set": "coffee",
    "title": "Water Handling"
  },
  "message": "R2 was retired at requirements/01-tank.shall.yaml:22."
}
```

## `GET /api/sets/{set}/search?q={words}`

What a reader typed names. A single word that names an item as an id, as
`resolve` reads it, comes first with `"match": "id"`. Then come the items of
the set whose id or title holds every word, with `"match": "title"`, and
those whose text does, with `"match": "text"`, each group in reading order,
ignoring case. At most 50 are listed, and `more` counts the rest. When
nothing is found, `message` says why.

```json
{
  "version": "dm7japskbab4.1",
  "query": "lamp",
  "results": [
    {
      "match": "title",
      "id": "R1",
      "state": "item",
      "set": "coffee",
      "title": "Refill Lamp",
      "file": "requirements/01-tank.shall.yaml"
    },
    {
      "match": "title",
      "id": "R1.1",
      "state": "item",
      "set": "coffee",
      "title": "Lamp Seen From The Door",
      "file": "requirements/01-tank.shall.yaml"
    }
  ]
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
