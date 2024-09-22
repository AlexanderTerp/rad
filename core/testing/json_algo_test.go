package testing

import "testing"

func TestJsonNonRootArrayExtraction(t *testing.T) {
	rsl := `
url = "https://google.com"

Id = json.id
Names = json.names

rad url:
    fields Id, Names
`

	setupAndRunCode(t, rsl, "--MOCK-RESPONSE", ".*:./test_json/not_root_array.json", "--NO-COLOR")
	expected := `Mocking response for url (matched ".*"): https://google.com
Id  Names                 
1   [Alice, Bob, Charlie]  
`
	assertOnlyOutput(t, stdOutBuffer, expected)
	assertNoErrors(t)
	resetTestState()
}

// todo flaky due to nondeterministic table ordering
func TestKeyExtraction(t *testing.T) {
	rsl := `
url = "https://google.com"

Name = json.results.*
Age = json.results.*.age
Hometown = json.results.*.hometown

rad url:
    fields Name, Age, Hometown
`

	setupAndRunCode(t, rsl, "--MOCK-RESPONSE", ".*:./test_json/unique_keys.json", "--NO-COLOR")
	expected := `Mocking response for url (matched ".*"): https://google.com
Name   Age  Hometown    
Alice  30   New York     
Bob    40   Los Angeles  
`
	assertOnlyOutput(t, stdOutBuffer, expected)
	assertNoErrors(t)
	resetTestState()
}

func TestKeyArrayExtraction(t *testing.T) {
	rsl := `
url = "https://google.com"

Hometown = json.*
Name = json.*[].name
Age = json.*[].age

rad url:
    fields Name, Age, Hometown
`

	setupAndRunCode(t, rsl, "--MOCK-RESPONSE", ".*:./test_json/unique_keys_array.json", "--NO-COLOR")
	expected := `Mocking response for url (matched ".*"): https://google.com
Name       Age  Hometown 
Alice      30   London    
Bob        40   London    
Charlotte  35   Paris     
David      25   Paris     
`
	assertOnlyOutput(t, stdOutBuffer, expected)
	assertNoErrors(t)
	resetTestState()
}

// todo flaky due to nondeterministic table ordering
func TestNestedWildcardExtraction(t *testing.T) {
	rsl := `
url = "https://google.com"

city = json.*
country = json.*.*[]
name = json.*.*[].name
age = json.*.*[].age

rad url:
    fields city, country, name, age
`

	setupAndRunCode(t, rsl, "--MOCK-RESPONSE", ".*:./test_json/nested_wildcard.json", "--NO-COLOR")
	expected := `Mocking response for url (matched ".*"): https://google.com
city  country    name       age 
York  Australia  Charlotte  35   
York  Australia  David      25   
York  Australia  Eve        20   
York  England    Alice      30   
York  England    Bob        40   
`
	assertOnlyOutput(t, stdOutBuffer, expected)
	assertNoErrors(t)
	resetTestState()
}
