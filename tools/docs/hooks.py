"""mkdocs hook: a link from the guide to a file outside it (an example,
the code) or to a scenario points at the repository on GitHub, since the
site holds only the guide's pages and pictures."""

import posixpath
import re

REPO = "https://github.com/telesapr/telesapr/blob/master/"
DOCS = "doc/guide"

_LINK = re.compile(r"\]\(((?:\.\./)+[^)#\s]*|scenarios/[^)#\s]*)")


def on_page_markdown(markdown, page, config, files):
    here = posixpath.join(DOCS, posixpath.dirname(page.file.src_uri))

    def repl(m):
        target = posixpath.normpath(posixpath.join(here, m.group(1)))
        inside = target.startswith(DOCS + "/") and not target.startswith(DOCS + "/scenarios/")
        if inside:
            return m.group(0)
        return "](" + REPO + target

    return _LINK.sub(repl, markdown)
