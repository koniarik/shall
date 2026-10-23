# `shall diff`

`shall diff` compares the sets under a directory as they stand at two git
refs, item by item, and prints what changed for people or, with `-json`, for
scripts.

```text
shall diff [-json] [-base ref] [-head ref] [directory]
```

- `-base` is the ref to compare from, `HEAD` unless given.
- `-head` is the ref to compare to. Unless given, it is the working tree as
  it stands on disk, files git does not track included.
- The directory is the current one unless given. It is read at the same
  place in the repository in each ref, together with the set files its
  manifests lead to elsewhere in the repository.

It exits with 0 when nothing changed, 1 when something did, and 2 when it
cannot compare: git cannot give a ref, neither ref holds a set under the
directory, or git cannot compare the words of a text.

What counts as a change is in
[diff.shall.yaml](requirements/diff.shall.yaml): sets are matched by the path
of their manifest and items by id; an item moved within its chapter, or a
value changed only in its whitespace, is no change.

The examples below compare the example set at the end of
[format.md](format.md) with a working tree in which R1.1 is reworded and
back to draft, and a chapter adds R3.

## Text

```text
shall.yaml: set coffee
  requirements/01-tank.shall.yaml
    R1 Refill Lamp: changed
      effective status: [-implemented-]{+draft+}
    R1.1 Lamp Seen From The Door: changed
      text: … refill lamp to anyone [-standing at-]{+entering+} the [-kitchen door.-]{+kitchen.+}
      status: [-implemented-]{+draft+}
  requirements/02-brew.shall.yaml: chapter added
    R3 No Brew On An Empty Tank: added
2 items changed, 1 added, 0 removed, 0 retired, 0 moved
```

Each set comes with its set-wide fields, then its files in path order, each
with its items in reading order and the items gone last, then the checker's
findings new at the head and gone from it. A changed value shows its removed
words `[-like this-]` and its added ones `{+like this+}`, with a few unchanged
words around each change; a list shows its entries so.

## JSON

- What is empty or absent is left out, except `sets`, which is `[]` when
  nothing changed.
- Values are as written: a text is a string, a list an array and a map an
  object. A field written `null`, or with nothing after its colon, is `null`;
  a field one side lacks has no `old` or no `new`.
- Paths are relative to the directory compared.

| Key | Meaning |
|---|---|
| `base`, `head` | The refs compared; an empty `head` is the working tree |
| `sets` | The sets that differ, in manifest path order |
| `totals` | How many items were `changed`, `added`, `removed`, `retired` and `moved`; a moved item also counts as what else happened to it |

A set carries:

| Key | Meaning |
|---|---|
| `manifest` | The path of its manifest, which matches it between the refs |
| `name` | The name its manifest gives it |
| `change` | `added`, `removed` or `changed` |
| `fields` | Its set-wide fields that differ: the manifest's `name`, `path` and `depends`, the mission file's `mission` and `ranking` |
| `files` | Its chapters added, removed or retitled, and its mission file added or removed |
| `items` | Its items that differ |
| `findings` | The checker's findings `new` at the head and those `gone`, each as `shall check` reports it, matched while ignoring their lines |

A field carries `field`, its name, with its `old` and `new` value.

A file carries `path`; `mission`, true for the mission file; `change`, which
is `added`, `removed` or `retitled`; and `title`, with its `old` and `new`
title.

An item carries:

| Key | Meaning |
|---|---|
| `id`, `title` | The item's id and title, at the head, or at the base for an item gone |
| `file` | The file holding it, at the head, or at the base for an item gone |
| `change` | `added`, `removed`, `retired` or `changed` |
| `movedFrom` | The file that held it at the base, when another |
| `fields` | The fields it writes whose values differ, without its children and retired ids, which are items of their own |
| `effectiveStatus` | A requirement's `old` and `new` effective status, where its written status does not tell the change, as when a child lowers its parent |

```json
{
  "base": "HEAD",
  "head": "",
  "sets": [
    {
      "manifest": "shall.yaml",
      "name": "coffee",
      "change": "changed",
      "files": [
        {
          "path": "requirements/02-brew.shall.yaml",
          "change": "added",
          "title": {
            "new": "Brewing"
          }
        }
      ],
      "items": [
        {
          "id": "R1",
          "title": "Refill Lamp",
          "file": "requirements/01-tank.shall.yaml",
          "change": "changed",
          "effectiveStatus": {
            "old": "implemented",
            "new": "draft"
          }
        },
        {
          "id": "R1.1",
          "title": "Lamp Seen From The Door",
          "file": "requirements/01-tank.shall.yaml",
          "change": "changed",
          "fields": [
            {
              "field": "text",
              "old": "The machine SHALL show the refill lamp to anyone standing at the kitchen door.",
              "new": "The machine SHALL show the refill lamp to anyone entering the kitchen."
            },
            {
              "field": "status",
              "old": "implemented",
              "new": "draft"
            }
          ]
        },
        {
          "id": "R3",
          "title": "No Brew On An Empty Tank",
          "file": "requirements/02-brew.shall.yaml",
          "change": "added"
        }
      ]
    }
  ],
  "totals": {
    "changed": 2,
    "added": 1,
    "removed": 0,
    "retired": 0,
    "moved": 0
  }
}
```
