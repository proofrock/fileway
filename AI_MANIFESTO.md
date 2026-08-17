# How AI is used in fileway

AI reads this codebase, and sometimes writes for it. It never merges into it.

_And yes, this document is written by AI, and reviewed and merged by an human._

Everything below follows from that one sentence. It is written down because
`fileway` moves other people's files, and because a tool that claims its builds
are reproducible owes you an answer about who wrote the code those builds come
from.

## Where AI is used

**Audit and review.** This is where it earns its place. An assistant can read
every file in the project in one sitting, which a human maintainer does rarely
and never twice the same way. It is used to hunt for defects: races, unbounded
allocations, error paths nobody exercises, documentation that no longer matches
the code.

The rule for findings is that a claim is not a finding. Every defect is reported
with the command that triggers it and the output that came back, run against a
server built from these sources. Where that was not possible — a race whose
window is nanoseconds wide, a browser behaviour taken from an RFC — the finding
says so explicitly, and is weighted accordingly. A report that mixes the two
without distinction is worse than no report, because it spends the maintainer's
trust on things that were never checked.

**Code, sometimes.** Remediation of those findings, regression tests, the
occasional refactor. Always as a proposal on a branch, never as a merge.

**Documentation.** Under the same rule as code.

## Where AI is not used

**No AI-authored change reaches `main` unamended.** Every line is read, rewritten
where it needs rewriting, re-tested and committed by hand, under the maintainer's
name and GPG signature. The git history of this project is human-authored because
the responsibility for it is human. If something in here is wrong, that is on the
maintainer, not on a tool.

**AI does not operate git.** Branches, commits, rebases, tags and pushes are the
maintainer's alone. An assistant that can rewrite history is an assistant that
can lose work; one that cannot touch it, cannot. When a git operation is needed,
the assistant writes down the commands and stops.

**Nothing lands on authority.** "The assistant says so" is not evidence, and
neither is a confident explanation. A proposed fix that cannot be demonstrated to
fix something does not become a commit.

## What has to be true before a merge

* `make test` is green — the Go tests under `-race`, plus the `bats` end-to-end
  suite that drives the real binary, the real protocol and the real CLI uploader.
* A fix arrives with a test that fails without it. A test that still passes when
  the fix is reverted is not a test, it is decoration; checking that is part of
  writing it.
* The reproducible build stays reproducible. `docs/server.adoc` explains how to
  rebuild the published binary from these sources and compare it byte for byte.
  That check means nothing if the sources contain code nobody read.

## Why the rule is this strict

The properties `fileway` is built on are all quiet ones. A download link is the
only thing protecting a transfer. Secrets sit in memory. The declared size of a
transfer decides how much the server allocates before a single byte arrives. The
build script exists to tie a binary to sources you can inspect.

Every one of those can be broken by a patch that reads perfectly well. The audit
of v0.9.0 found exactly that class of defect, including one in the script whose
whole purpose was guaranteeing reproducibility, where a single missing `export`
had silently disabled the setting it was written to enforce — for months, in code
that looked right.

That cuts both ways, and it is the honest argument for this arrangement rather
than against it. An assistant is unusually good at finding defects of that shape,
because finding them is a reading problem and reading is what it does best. It is
not the thing that gets to decide they are fixed, because deciding is not a
reading problem: it needs someone who will still be here when the decision turns
out to be wrong.

## If you use or contribute to fileway

Every commit in this repository was reviewed and signed by a human, whatever
helped write it. Contributions are welcome under the same terms: use whatever
tools you like to write them, and be prepared to defend every line as your own,
because that is what opening a pull request says.

---

*This file describes actual practice, not an aspiration. If the practice changes,
this file changes with it — and if you find code in `main` that contradicts it,
that is a bug worth reporting.*
