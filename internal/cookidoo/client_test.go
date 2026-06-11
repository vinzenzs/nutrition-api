package cookidoo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRecipeURL(t *testing.T) {
	valid := []string{
		"https://cookidoo.de/recipes/recipe/de-DE/r386806",
		"https://cookidoo.de/recipes/recipe/de-DE/r386806/",
		"https://cookidoo.com/recipes/recipe/en-US/r123",
		"https://cookidoo.co.uk/recipes/recipe/en-GB/r999",
		"http://cookidoo.ch/recipes/recipe/de-CH/r1",
	}
	for _, u := range valid {
		assert.Truef(t, IsRecipeURL(u), "expected valid: %s", u)
	}

	invalid := []string{
		"https://evil.example.com/recipes/recipe/de-DE/r1",
		"https://cookidoo.de.evil.com/recipes/recipe/de-DE/r1",
		"https://cookidoo.de/collection/de-DE/p/x",     // not a recipe path
		"https://cookidoo.de/recipes/recipe/de-DE",     // missing id segment
		"https://notcookidoo.de/recipes/recipe/x/y",    // wrong host
		"ftp://cookidoo.de/recipes/recipe/de-DE/r1",     // wrong scheme
		"not a url at all",
		"",
	}
	for _, u := range invalid {
		assert.Falsef(t, IsRecipeURL(u), "expected invalid: %s", u)
	}
}

func TestFetch_InvalidURLNoNetworkCall(t *testing.T) {
	// A client whose transport panics proves Fetch returns before any network
	// activity when the URL is not a Cookidoo recipe URL.
	c := New(Config{HTTPClient: &http.Client{Transport: panicTransport{}}})
	_, err := c.Fetch(context.Background(), "https://evil.example.com/x")
	assert.ErrorIs(t, err, ErrNotCookidooURL)
}

func TestFetchAndParse_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureHTML))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client()})
	r, err := c.fetchAndParse(context.Background(), srv.URL+"/recipes/recipe/de-DE/r386806")
	require.NoError(t, err)
	assert.Equal(t, "Vegetarische Linsen-Lasagne", r.Name)
	assert.Len(t, r.Ingredients, 4)
}

func TestFetchAndParse_Non2xxIsFetchFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client()})
	_, err := c.fetchAndParse(context.Background(), srv.URL+"/x")
	var ff *ErrFetchFailed
	require.True(t, errors.As(err, &ff), "want *ErrFetchFailed, got %v", err)
	assert.Equal(t, http.StatusNotFound, ff.StatusCode)
}

func TestFetchAndParse_PageWithoutRecipeIsNoJSONLD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>no recipe here</body></html>`))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client()})
	_, err := c.fetchAndParse(context.Background(), srv.URL+"/x")
	assert.ErrorIs(t, err, ErrNoRecipeJSONLD)
}

type panicTransport struct{}

func (panicTransport) RoundTrip(*http.Request) (*http.Response, error) {
	panic("network call attempted for an invalid URL")
}
