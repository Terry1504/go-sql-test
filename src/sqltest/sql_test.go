package sqltest

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/ziutek/mymysql/godrv"
)

type Tester interface {
	RunTest(*testing.T, func(params))
}

type mysqlDB int
type sqliteDB int

var (
	mysql  = mysqlDB(1)
	sqlite = sqliteDB(1)
)

type params struct {
	dbType Tester
	*testing.T
	*sql.DB
}

func (t params) mustExec(sql string, args ...interface{}) sql.Result {
	res, err := t.DB.Exec(sql, args...)
	if err != nil {
		t.Fatalf("Error running %q: %v", sql, err)
	}
	return res
}

func (sqliteDB) RunTest(t *testing.T, fn func(params)) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "foo.db"))
	if err != nil {
		t.Fatalf("foo.db open fail: %v", err)
	}
	fn(params{sqlite, t, db})
}

func (mysqlDB) RunTest(t *testing.T, fn func(params)) {
	user := os.Getenv("GOSQLTEST_MYSQL_USER")
	if user == "" {
		user = "root"
	}
	pass, err := os.Getenverror("GOSQLTEST_MYSQL_PASS")
	if err != nil {
		pass = "root"
	}
	dbName := "gosqltest"
	db, err := sql.Open("mymysql", fmt.Sprintf("%s/%s/%s", dbName, user, pass))
	if err != nil {
		t.Fatalf("error connecting: %v", err)
	}

	params := params{mysql, t, db}

	// Drop all tables in the test database.
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		t.Fatalf("failed to enumerate tables: %v", err)
	}
	for rows.Next() {
		var table string
		if rows.Scan(&table) == nil {
			params.mustExec("DROP TABLE " + table)
		}
	}

	fn(params)
}

func sqlBlobParam(t params, size int) string {
	if t.dbType == sqlite {
		return fmt.Sprintf("blob[%d]", size)
	}
	return fmt.Sprintf("VARBINARY(%d)", size)
}

func TestBlobs_SQLite(t *testing.T) { sqlite.RunTest(t, testBlobs) }
func TestBlobs_MySQL(t *testing.T)  { mysql.RunTest(t, testBlobs) }

func testBlobs(t params) {
	var blob = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	t.mustExec("create table foo (id integer primary key, bar " + sqlBlobParam(t, 16) + ")")
	t.mustExec("replace into foo (id, bar) values(?,?)", 0, blob)

	want := fmt.Sprintf("%x", blob)

	b := make([]byte, 16)
	err := t.QueryRow("select bar from foo where id = ?", 0).Scan(&b)
	got := fmt.Sprintf("%x", b)
	if err != nil {
		t.Errorf("[]byte scan: %v", err)
	} else if got != want {
		t.Errorf("for []byte, got %q; want %q", got, want)
	}

	err = t.QueryRow("select bar from foo where id = ?", 0).Scan(&got)
	want = string(blob)
	if err != nil {
		t.Errorf("string scan: %v", err)
	} else if got != want {
		t.Errorf("for string, got %q; want %q", got, want)
	}
}

func TestManyQueryRow_SQLite(t *testing.T) { sqlite.RunTest(t, testManyQueryRow) }
func TestManyQueryRow_MySQL(t *testing.T)  { mysql.RunTest(t, testManyQueryRow) }

func testManyQueryRow(t params) {
	t.mustExec("create table foo (id integer primary key, name varchar(50))")
	t.mustExec("insert into foo (id, name) values(?,?)", 1, "bob")
	var name string
	for i := 0; i < 10000; i++ {
		err := t.QueryRow("select name from foo where id = ?", 1).Scan(&name)
		if err != nil || name != "bob" {
			t.Fatalf("on query %d: err=%v, name=%q", i, err, name)
		}
	}
}
