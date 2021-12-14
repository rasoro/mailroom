package testsuite

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/nyaruka/gocommon/storage"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"

	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

/*var tableHashes = map[string]string{
	"channels_channel": "3587399bad341401f1880431c0bc772a",
	"contacts_contact": "0382ef6e58e260c0c76dcc84550e6793",
	"orgs_org":         "0f650bf7b9fb77ffa3ff0992be98da53",
	"tickets_ticketer": "6487a4aed61e16c3aa0d6cf117f58de3",
}*/

const MediaStorageDir = "_test_media_storage"
const SessionStorageDir = "_test_session_storage"

// Refresh is our type for the pieces of org assets we want fresh (not cached)
type ResetFlag int

// refresh bit masks
const (
	ResetAll     = ResetFlag(^0)
	ResetDB      = ResetFlag(1 << 1)
	ResetData    = ResetFlag(1 << 2)
	ResetRedis   = ResetFlag(1 << 3)
	ResetStorage = ResetFlag(1 << 4)
)

// Reset clears out both our database and redis DB
func Reset(what ResetFlag) {
	if what&ResetDB > 0 {
		resetDB()
	} else if what&ResetData > 0 {
		resetData()
	}
	if what&ResetRedis > 0 {
		resetRedis()
	}
	if what&ResetStorage > 0 {
		resetStorage()
	}

	models.FlushCache()
	logrus.SetLevel(logrus.DebugLevel)
}

// Get returns the various runtime things a test might need
func Get() (context.Context, *runtime.Runtime, *sqlx.DB, *redis.Pool) {
	db := getDB()
	rp := getRP()
	rt := &runtime.Runtime{
		DB:             db,
		ReadonlyDB:     db,
		RP:             rp,
		ES:             nil,
		MediaStorage:   storage.NewFS(MediaStorageDir),
		SessionStorage: storage.NewFS(SessionStorageDir),
		Config:         runtime.NewDefaultConfig(),
	}

	/*for name, expected := range tableHashes {
		var actual string
	    must(db.Get(&actual, fmt.Sprintf(`SELECT md5(array_to_string(array_agg(t.* order by id), '|', '')) FROM %s t`, name)))
		if actual != expected {
			panic(fmt.Sprintf("table has mismatch for %s, expected: %s, got %s", name, expected, actual))
		}
	}*/

	return context.Background(), rt, db, rp
}

// returns an open test database pool
func getDB() *sqlx.DB {
	return sqlx.MustOpen("postgres", "postgres://mailroom_test:temba@localhost/mailroom_test?sslmode=disable&Timezone=UTC")
}

// returns a redis pool to our test database
func getRP() *redis.Pool {
	return &redis.Pool{
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", "localhost:6379")
			if err != nil {
				return nil, err
			}
			_, err = conn.Do("SELECT", 0)
			return conn, err
		},
	}
}

// returns a redis connection, Close() should be called on it when done
func getRC() redis.Conn {
	conn, err := redis.Dial("tcp", "localhost:6379")
	noError(err)
	_, err = conn.Do("SELECT", 0)
	noError(err)
	return conn
}

// resets our database to our base state from our RapidPro dump
//
// mailroom_test.dump can be regenerated by running:
//   % python manage.py mailroom_db
//
// then copying the mailroom_test.dump file to your mailroom root directory
//   % cp mailroom_test.dump ../mailroom
func resetDB() {
	db := getDB()
	defer db.Close()

	db.MustExec("drop owned by mailroom_test cascade")
	dir, _ := os.Getwd()

	// our working directory is set to the directory of the module being tested, we want to get just
	// the portion that points to the mailroom directory
	for !strings.HasSuffix(dir, "mailroom") && dir != "/" {
		dir = path.Dir(dir)
	}

	mustExec("pg_restore", "-h", "localhost", "-d", "mailroom_test", "-U", "mailroom_test", path.Join(dir, "./mailroom_test.dump"))
}

// resets our redis database
func resetRedis() {
	rc, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		panic(fmt.Sprintf("error connecting to redis db: %s", err.Error()))
	}
	rc.Do("SELECT", 0)
	_, err = rc.Do("FLUSHDB")
	if err != nil {
		panic(fmt.Sprintf("error flushing redis db: %s", err.Error()))
	}
}

// clears our storage for tests
func resetStorage() {
	must(os.RemoveAll(MediaStorageDir))
	must(os.RemoveAll(SessionStorageDir))
}

var resetDataSQL = `
DELETE FROM notifications_notification;
DELETE FROM notifications_incident;
DELETE FROM request_logs_httplog;
DELETE FROM tickets_ticketevent;
DELETE FROM tickets_ticket;
DELETE FROM triggers_trigger_contacts WHERE trigger_id >= 30000;
DELETE FROM triggers_trigger_groups WHERE trigger_id >= 30000;
DELETE FROM triggers_trigger WHERE id >= 30000;
DELETE FROM channels_channelcount;
DELETE FROM msgs_msg;
DELETE FROM flows_flowpathrecentrun;
DELETE FROM flows_flowrun;
DELETE FROM flows_flowsession;
DELETE FROM flows_flowrevision WHERE flow_id >= 30000;
DELETE FROM flows_flow WHERE id >= 30000;
DELETE FROM campaigns_eventfire;
DELETE FROM flows_flowrun;
DELETE FROM flows_flowsession;
DELETE FROM flows_flowrevision WHERE id >= 30000;
DELETE FROM flows_flow WHERE id >= 30000;
DELETE FROM contacts_contactimportbatch;
DELETE FROM contacts_contactimport;
DELETE FROM contacts_contacturn WHERE id >= 30000;
DELETE FROM contacts_contactgroup_contacts WHERE contact_id >= 30000 OR contactgroup_id >= 30000;
DELETE FROM contacts_contact WHERE id >= 30000;
DELETE FROM contacts_contactgroupcount WHERE group_id >= 30000;
DELETE FROM contacts_contactgroup WHERE id >= 30000;

ALTER SEQUENCE flows_flow_id_seq RESTART WITH 30000;
ALTER SEQUENCE tickets_ticket_id_seq RESTART WITH 1;
ALTER SEQUENCE msgs_msg_id_seq RESTART WITH 1;
ALTER SEQUENCE contacts_contact_id_seq RESTART WITH 30000;
ALTER SEQUENCE contacts_contacturn_id_seq RESTART WITH 30000;
ALTER SEQUENCE contacts_contactgroup_id_seq RESTART WITH 30000;`

// removes contact data not in the test database dump. Note that this function can't
// undo changes made to the contact data in the test database dump.
func resetData() {
	db := getDB()
	defer db.Close()

	db.MustExec(resetDataSQL)

	// because groups have changed
	models.FlushCache()
}

// utility function for running a command panicking if there is any error
func mustExec(command string, args ...string) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("error restoring database: %s: %s", err, string(output)))
	}
}

// convenience way to call a func and panic if it errors, e.g. must(foo())
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// if just checking an error is nil noError(err) reads better than must(err)
var noError = must

func ReadFile(path string) []byte {
	d, err := os.ReadFile(path)
	noError(err)
	return d
}
