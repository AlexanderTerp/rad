package testing

import "testing"

func TestRadColorStmts(t *testing.T) {
	rsl := `
url = "https://google.com"
id = json[].id
name = json[].name
rad url:
	fields id, name
	name:
		color red "Alice"
`
	setupAndRunCode(t, rsl, "--MOCK-RESPONSE", ".*:./responses/id_name.json", "--NO-COLOR")
	expected := `id  name
1   Alice
2   Bob
`
	assertOutput(t, stdOutBuffer, expected)
	assertOutput(t, stdErrBuffer, "Mocking response for url (matched \".*\"): https://google.com\n")
	assertNoErrors(t)
	resetTestState()
}
