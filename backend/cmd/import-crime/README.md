# Boston incident importer

This one-time/batch command loads only offenses marked `include=true` in
`backend/data/boston_offense_mapping.csv`. It skips included incidents without
valid coordinates. Offenses excluded by the mapping are not inserted. An
unmapped offense code aborts the transaction so a new source code cannot be
silently included or discarded.

Apply `migrations/migrations/006_create_crime_incidents.up.sql` after migration
005, then run from the `backend` directory:

```sh
DATABASE_URL="${DATABASE_URL:?Set DATABASE_URL first}" \
go run ./cmd/import-crime \
  -csv ../tmpivat4nah.csv \
  -mapping data/boston_offense_mapping.csv \
  -snapshot-date YYYY-MM-DD
```

Replace `YYYY-MM-DD` with the date this local source snapshot was downloaded.
The command wraps the import in one transaction and upserts by source, incident
number, and offense code. The CSV and mapping paths can be changed with flags.
