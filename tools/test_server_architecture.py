"""Regression tests for the Go server architecture guard."""

import contextlib
import io
from pathlib import Path
import tempfile
import unittest

from server_architecture import (
    AGENT_MODULE,
    DependencyException,
    INTERNAL_PREFIX,
    check_server_architecture,
)


class ServerArchitectureTests(unittest.TestCase):
    def fixture(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        root = Path(directory.name)
        self.write(root, "server/internal/domain/domain.go", "package domain\n")
        self.write(root, "server/internal/application/application.go", '''package application
import (
    "github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
    "github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)
''')
        self.write(root, "server/internal/csvimport/importer.go", '''package csvimport
import "github.com/shuichirohayafuji/spendable-today/server/internal/domain"
''')
        self.write(root, "server/internal/agentadapter/adapter.go", '''package agentadapter
import (
    "github.com/shuichiro-hayafuji/arran_agent"
    "github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)
''')
        self.write(root, "server/internal/app/app.go", '''package app
import (
    "github.com/shuichiro-hayafuji/arran_agent"
    "github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/openai"
    "github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/postgres"
)
''')
        self.write(root, "server/internal/infrastructure/openai/client.go", '''package openai
import "github.com/shuichiro-hayafuji/arran_agent"
''')
        self.write(root, "server/internal/infrastructure/persistence/postgres/repository.go", '''package postgres
import "github.com/shuichirohayafuji/spendable-today/server/internal/domain"
''')
        self.write(root, "server/agent/agent.go", '''package agent
type Transaction struct {
    TransactionDate string
    Amount int64
    Category string
}
''')
        return root

    @staticmethod
    def write(root, name, text):
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def check(self, root, exceptions=()):
        with contextlib.redirect_stdout(io.StringIO()) as output:
            result = check_server_architecture(root, exceptions)
        return result, output.getvalue()

    def test_allowed_dependencies_and_transaction_fields_pass(self):
        self.assertEqual(self.check(self.fixture())[0], "PASS")

    def test_comments_and_raw_strings_do_not_create_imports(self):
        root = self.fixture()
        self.write(root, "server/internal/domain/domain.go", '''package domain
// import "net/http"
const example = `
import "database/sql"
`
''')
        self.assertEqual(self.check(root)[0], "PASS")

    def test_domain_cannot_import_http_database_or_agent(self):
        for imported in ("net/http", "database/sql", AGENT_MODULE):
            with self.subTest(imported=imported):
                root = self.fixture()
                self.write(root, "server/internal/domain/domain.go", f'''package domain
import "{imported}"
''')
                result, output = self.check(root)
                self.assertEqual(result, "FAIL")
                self.assertIn("A-S01", output)

    def test_agent_cannot_import_parent_internal(self):
        root = self.fixture()
        self.write(root, "server/agent/agent.go", f'''package agent
import "{INTERNAL_PREFIX}domain"
type Transaction struct {{
    TransactionDate string
    Amount int64
    Category string
}}
''')
        result, output = self.check(root)
        self.assertEqual(result, "FAIL")
        self.assertIn("A-S04", output)

    def test_unauthorized_layer_cannot_import_implementations(self):
        for imported in (
            INTERNAL_PREFIX + "infrastructure/openai",
            "github.com/jackc/pgx/v5",
            "github.com/openai/openai-go",
        ):
            with self.subTest(imported=imported):
                root = self.fixture()
                self.write(root, "server/internal/handler/handler.go", f'''package handler
import "{imported}"
''')
                result, output = self.check(root)
                self.assertEqual(result, "FAIL")
                self.assertIn("A-S05", output)

    def test_application_cannot_construct_agent_adapter(self):
        cases = (
            (INTERNAL_PREFIX + "agentadapter", "A-S02"),
            ("database/sql", "A-S05"),
            ("net/http", "A-S02"),
        )
        for imported, rule in cases:
            with self.subTest(imported=imported):
                root = self.fixture()
                self.write(root, "server/internal/application/application.go", f'''package application
import "{imported}"
''')
                result, output = self.check(root)
                self.assertEqual(result, "FAIL")
                self.assertIn(rule, output)

    def test_new_internal_package_requires_a_dependency_policy(self):
        root = self.fixture()
        self.write(root, "server/internal/newfeature/newfeature.go", "package newfeature\n")
        result, output = self.check(root)
        self.assertEqual(result, "FAIL")
        self.assertIn("no registered dependency policy", output)

    def test_agent_transaction_cannot_gain_unapproved_fields(self):
        for field in ("Merchant string", "merchant string", "Merchant, Account string"):
            with self.subTest(field=field):
                root = self.fixture()
                self.write(root, "server/agent/agent.go", f'''package agent
type Transaction struct {{
    TransactionDate string
    Amount int64
    Category string
    {field}
}}
''')
                result, output = self.check(root)
                self.assertEqual(result, "FAIL")
                self.assertIn("A-S03/B-S08", output)

    def test_named_exception_is_exact_and_does_not_hide_other_imports(self):
        root = self.fixture()
        imported = INTERNAL_PREFIX + "infrastructure/openai"
        self.write(root, "server/internal/handler/handler.go", f'''package handler
import "{imported}"
''')
        exception = DependencyException("A-S05", "internal/handler", imported, "EX-TEST")
        self.assertEqual(self.check(root, (exception,))[0], "PASS")
        self.write(root, "server/internal/handler/handler.go", f'''package handler
import (
    "{imported}"
    "{INTERNAL_PREFIX}infrastructure/persistence/postgres"
)
''')
        self.assertEqual(self.check(root, (exception,))[0], "FAIL")

    def test_missing_agent_is_blocked(self):
        root = self.fixture()
        for path in (root / "server/agent").iterdir():
            path.unlink()
        (root / "server/agent").rmdir()
        self.assertEqual(self.check(root)[0], "BLOCKED")


if __name__ == "__main__":
    unittest.main()
