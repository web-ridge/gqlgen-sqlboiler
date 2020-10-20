// testing really need more improvements!
// Please add more tests if you use this

package gbgen

import "testing"

func TestShortType(t *testing.T) {
	testShortType(t, "gitlab.com/product/app/backend/graphql_models.FlowWhere", "FlowWhere")
	testShortType(t, "*gitlab.com/product/app/backend/graphql_models.FlowWhere", "*FlowWhere")
	testShortType(t, "*github.com/web-ridge/go-utils/boilergql/boilergql.GeoPoint", "*GeoPoint")
	testShortType(t, "github.com/web-ridge/go-utils/boilergql/boilergql.GeoPoint", "GeoPoint")
	testShortType(t, "*string", "*string")
	testShortType(t, "string", "string")
	testShortType(t, "*time.Time", "*time.Time")
}

func testShortType(t *testing.T, input, output string) {
	result := getShortType(input)
	if result != output {
		t.Errorf("%v should result in %v but did result in %v", input, output, result)
	}
}
