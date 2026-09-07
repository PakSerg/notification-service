package repository

import (
	"reflect"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "single statement",
			sql:  `CREATE TABLE IF NOT EXISTS t (id TEXT PRIMARY KEY)`,
			want: []string{`CREATE TABLE IF NOT EXISTS t (id TEXT PRIMARY KEY)`},
		},
		{
			name: "multiple statements with blank lines, as in a migration file",
			sql: `
				CREATE TABLE IF NOT EXISTS notifications (
				    id TEXT PRIMARY KEY
				);

				CREATE INDEX IF NOT EXISTS idx_status ON notifications (status);
				CREATE INDEX IF NOT EXISTS idx_created_at ON notifications (created_at);
			`,
			want: []string{
				"CREATE TABLE IF NOT EXISTS notifications (\n\t\t\t\t    id TEXT PRIMARY KEY\n\t\t\t\t)",
				"CREATE INDEX IF NOT EXISTS idx_status ON notifications (status)",
				"CREATE INDEX IF NOT EXISTS idx_created_at ON notifications (created_at)",
			},
		},
		{
			name: "trailing semicolon and surrounding whitespace produce no empty statements",
			sql:  "  ALTER TABLE t ADD COLUMN a INTEGER;  \n",
			want: []string{"ALTER TABLE t ADD COLUMN a INTEGER"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitStatements(tc.sql)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %#v, got %#v", tc.want, got)
			}
		})
	}
}
