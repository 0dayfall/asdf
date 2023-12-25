package rest

import (
	"asdf/internal/db"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
)

func TestFinger(t *testing.T) {
	dir, _ := os.Getwd()
	fmt.Println("Current working directory:", dir)
	db := db.NewData()
	err := db.LoadData(path.Join("test", "data.json"))
	if err != nil {
		t.Errorf("Setup failed %v", err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/.well-known/webfinger?resource=acct%3Agreenorangebay%40yahoo.com", nil)
	if err != nil {
		t.Errorf("Could not create a new request: %v", err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	wfh := WebFingerHandler{Data: db}
	wfh.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := `{"subject":"acct:greenorangebay@yahoo.com","aliases":["dan@ekstrom.com"],"links":[{"rel":"Social media","type":"text/html","href":"http://www.facebook.com/dans"},{"rel":"http://webfinger.net/rel/avatar","type":"image/png","href":"https://www.examplesocial.com/avatars/dan.png"}],"properties":{"http://www.facebook.com/dans":"Facebook page"}}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
