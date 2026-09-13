module github.com/shuichirohayafuji/spendable-today/server

go 1.26

require golang.org/x/text v0.38.0

require github.com/shuichiro-hayafuji/arran_agent v0.0.0

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.21.0 // indirect
)

// server/agent is supplied by the arran_agent Git submodule. Keep this local
// replacement so the parent always tests the checked-out submodule revision.
replace github.com/shuichiro-hayafuji/arran_agent => ./agent
