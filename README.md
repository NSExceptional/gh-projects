# gh-projects

A [`gh`](https://cli.github.com) extension for working with **GitHub Projects (v2)** from the
command line — built to be concise enough for both humans and agents.

The built-in `gh project` commands can read and write Projects, but every
mutation makes you resolve a chain of opaque node IDs first (the project's
`PVT_…` id, the item's `PVTI_…` id, the field's id, and an option hash like
`df73e18b`), there's no convenience for "what's ready to work on," and you
can't toggle a task-list checkbox at all. `gh-projects` resolves those IDs for
you and adds the project-management verbs you actually use:

```sh
gh projects ready 4                         # items in the "Todo" column
gh projects move 4 12 "Needs review"        # by issue number + column name
gh projects check 4 12 "write tests"        # tick a checkbox in the issue body
gh projects create 4 --title "Add caching" --label enhancement
```

You address **projects by number**, **items by issue number**, and
**columns/fields by name**. It reuses `gh`'s authentication, so there are no
tokens to manage.

## Install

```sh
gh extension install NSExceptional/gh-projects
```

Upgrade with `gh extension upgrade gh-projects`.

### Authentication scopes

Reading needs `read:project`; writing needs `project`:

```sh
gh auth refresh -s project     # read + write
```

## Usage

`gh projects <command> [<project-number>] [flags]`

The project owner defaults to the authenticated user; pass `--owner <login>`
for organizations or other users. Most read commands accept `--json` for
machine-readable output.

### Reading

| Command | Description |
| --- | --- |
| `list` | List projects (number, title, item count, linked repos) |
| `view <project>` | Project details, columns, field list, counts |
| `board <project>` | Items grouped by their `Status` column |
| `items <project> [--status NAME]` | List items, optionally filtered by column |
| `ready <project>` | Shortcut for `items --status Todo` |
| `fields <project>` | Field definitions and single-select options |
| `links <project>` | Repositories the project is linked to |

### Items

| Command | Description |
| --- | --- |
| `create <project> --title T [--repo O/R] [--body B] [--label L]… [--status S]` | Create a **real issue** and add it to the board (default way to make a task). `--repo` defaults to the project's sole linked repo. |
| `draft <project> --title T [--body B] [--status S]` | Add a draft issue (project-only) |
| `add <project> <issue-url-or-#> [--repo O/R]` | Add an existing issue/PR |
| `rm <project> <issue-#-or-item-id>` | Remove an item from the project (does not delete the issue) |
| `convert <project> <draft-title-or-item-id> --repo O/R` | Convert a draft into a real issue |

### Field values & movement

| Command | Description |
| --- | --- |
| `move <project> <issue-#> <column>` | Set the `Status` field (move between columns) |
| `set <project> <issue-#> <field> <value>` | Set any field value (single-select by option name, plus text/number/date/iteration) |
| `check <project> <issue-#> <task-text>` | Tick the task-list box whose text uniquely matches |
| `uncheck <project> <issue-#> <task-text>` | Untick it |

### Field definitions & repo links

| Command | Description |
| --- | --- |
| `field-create <project> --name N --type TYPE [--option O]…` | Create a field (`text`, `number`, `date`, `single_select`) |
| `field-delete <project> <field-name>` | Delete a field |
| `link <project> <owner/repo>` | Link the project to a repo (so it appears on that repo's Projects tab) |
| `unlink <project> <owner/repo>` | Remove that link |

## Notes

- A project is linked to **zero or more** repositories (it's many-to-many);
  that link is what makes a project appear on a repo's Projects tab. Manage it
  with `link` / `unlink` / `links`. (The web UI's "default repository" setting,
  which only pre-selects a repo in dropdowns, is not exposed by GitHub's public
  API, so it can't be read or changed here.)
- `check`/`uncheck` require the matched task text to be **unique** within the
  body; they error rather than risk toggling the wrong box.
- **Read-after-write lag:** GitHub's project *items* connection is eventually
  consistent, so an item you just created may take a few seconds to show up in
  `items`/`board`. The write commands themselves use the returned IDs directly
  and are unaffected.

## Development

```sh
go build -o gh-projects .     # build the binary
go test ./...                 # fast unit tests (no network)
```

Live integration tests exercise the real API and are gated behind a build tag.
They need gh authenticated with the `project` scope and operate on a disposable
project, cleaning up everything they create:

```sh
# link/unlink, set all field types, add existing item, draft body toggle
GHP_TEST_PROJECT=5 go test -tags integration -run Integration ./internal/projects -v

# also the issue-creating paths (create/convert); point at a throwaway repo
GHP_TEST_PROJECT=5 GHP_TEST_ISSUE_REPO=you/throwaway \
  go test -tags integration -run Integration ./internal/projects -v
```

Configuration env vars: `GHP_TEST_OWNER` (default: authed user), `GHP_TEST_PROJECT`
(default: 5), `GHP_TEST_REPO` (default: `NSExceptional/home`, only referenced not
modified), `GHP_TEST_ISSUE_REPO` (required to run create/convert tests).

## License

MIT — see [LICENSE](LICENSE).
