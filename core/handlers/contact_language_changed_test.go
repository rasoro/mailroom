package handlers_test

import (
	"testing"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/actions"
	"github.com/nyaruka/mailroom/core/handlers"
	"github.com/nyaruka/mailroom/testsuite/testdata"
)

func TestContactLanguageChanged(t *testing.T) {
	tcs := []handlers.TestCase{
		{
			Actions: handlers.ContactActionMap{
				testdata.Cathy.ID: []flows.Action{
					actions.NewSetContactLanguage(handlers.NewActionUUID(), "fra"),
					actions.NewSetContactLanguage(handlers.NewActionUUID(), "eng"),
				},
				testdata.George.ID: []flows.Action{
					actions.NewSetContactLanguage(handlers.NewActionUUID(), "spa"),
				},
				testdata.Alexandria.ID: []flows.Action{
					actions.NewSetContactLanguage(handlers.NewActionUUID(), ""),
				},
			},
			SQLAssertions: []handlers.SQLAssertion{
				{
					SQL:   "select count(*) from contacts_contact where id = $1 and language = 'eng'",
					Args:  []interface{}{testdata.Cathy.ID},
					Count: 1,
				},
				{
					SQL:   "select count(*) from contacts_contact where id = $1 and language = 'spa'",
					Args:  []interface{}{testdata.George.ID},
					Count: 1,
				},
				{
					SQL:   "select count(*) from contacts_contact where id = $1 and language is NULL;",
					Args:  []interface{}{testdata.Bob.ID},
					Count: 1,
				},
				{
					SQL:   "select count(*) from contacts_contact where id = $1 and language is NULL;",
					Args:  []interface{}{testdata.Alexandria.ID},
					Count: 1,
				},
			},
		},
	}

	handlers.RunTestCases(t, tcs)
}
