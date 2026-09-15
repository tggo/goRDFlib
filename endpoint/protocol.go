package endpoint

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/tggo/goRDFlib/internal/iri"
)

// Media types of the SPARQL Protocol.
const (
	mtForm        = "application/x-www-form-urlencoded"
	mtSPARQLQuery = "application/sparql-query"
	mtUpdate      = "application/sparql-update"
)

// protocolRequest is a SPARQL Protocol request after reading, before
// evaluation.
type protocolRequest struct {
	isUpdate bool
	text     string
	// Query dataset (default-graph-uri, named-graph-uri) or update dataset
	// (using-graph-uri, using-named-graph-uri).
	defaultGraphs []string
	namedGraphs   []string
}

// sparql serves the SPARQL Protocol on the endpoint URL.
func (x *exchange) sparql(allowQuery, allowUpdate bool) error {
	r := x.r
	allow := "GET, HEAD, POST"
	if !allowQuery {
		allow = "POST"
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if !allowQuery {
			return methodNotAllowed(r.Method, allow, "send updates with POST")
		}
	case http.MethodPost:
	case http.MethodOptions:
		x.w.Header().Set("Allow", allow)
		x.w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return methodNotAllowed(r.Method, allow, "the SPARQL endpoint accepts GET and POST")
	}

	req, err := x.readProtocolRequest()
	if err != nil {
		return err
	}
	if req == nil {
		// GET without a query: the service description.
		if err := x.authorize(OpQuery); err != nil {
			return err
		}
		return x.serviceDescription()
	}
	if req.isUpdate && !allowUpdate {
		return httpErrf(http.StatusBadRequest, "this URL serves queries only; send updates to the update endpoint")
	}
	if !req.isUpdate && !allowQuery {
		return httpErrf(http.StatusBadRequest, "this URL serves updates only; send queries to the query endpoint")
	}
	if req.isUpdate {
		if err := x.authorize(OpUpdate); err != nil {
			return err
		}
		if x.h.cfg.readOnly {
			return httpErr(http.StatusForbidden, ErrReadOnly)
		}
		return x.update(req)
	}
	if err := x.authorize(OpQuery); err != nil {
		return err
	}
	return x.query(req)
}

func methodNotAllowed(method, allow, hint string) error {
	return &HTTPError{
		Status: http.StatusMethodNotAllowed,
		Header: http.Header{"Allow": {allow}},
		Err:    fmt.Errorf("method %s not allowed: %s", method, hint),
	}
}

// readProtocolRequest extracts the operation from the request. It returns
// nil, nil for a GET without query (the service description).
func (x *exchange) readProtocolRequest() (*protocolRequest, error) {
	r := x.r
	urlParams := r.URL.Query()

	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		if _, ok := urlParams["update"]; ok {
			return nil, methodNotAllowed(r.Method, "POST", "SPARQL updates must be sent with POST (SPARQL 1.1 Protocol §2.2)")
		}
		if _, ok := urlParams["query"]; !ok {
			return nil, nil
		}
		return protocolParams(urlParams, false)
	}

	mt, charset := mediaType(r.Header.Get("Content-Type"))
	switch mt {
	case mtForm:
		body, err := x.readBody(x.h.cfg.maxRequestBytes)
		if err != nil {
			return nil, err
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, httpErrf(http.StatusBadRequest, "malformed form body: %v", err)
		}
		// URL parameters count too: a query in both places is two queries.
		for k, vs := range urlParams {
			form[k] = append(form[k], vs...)
		}
		_, hasQuery := form["query"]
		_, hasUpdate := form["update"]
		switch {
		case hasQuery && hasUpdate:
			return nil, httpErrf(http.StatusBadRequest, "a request carries either query= or update=, not both")
		case hasQuery:
			return protocolParams(form, false)
		case hasUpdate:
			return protocolParams(form, true)
		}
		return nil, httpErrf(http.StatusBadRequest, "missing query= or update= parameter in the form body")
	case mtSPARQLQuery, mtUpdate:
		if !utf8Charset(charset) {
			return nil, httpErrf(http.StatusUnsupportedMediaType, "charset %q is not supported: SPARQL requests must be UTF-8", charset)
		}
		isUpdate := mt == mtUpdate
		for _, k := range []string{"query", "update"} {
			if _, ok := urlParams[k]; ok {
				return nil, httpErrf(http.StatusBadRequest, "a %s body must not be combined with a %s= URL parameter", mt, k)
			}
		}
		body, err := x.readBody(x.h.cfg.maxRequestBytes)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(body) {
			return nil, httpErrf(http.StatusBadRequest, "request body is not valid UTF-8")
		}
		req, err := protocolParams(urlParams, isUpdate)
		if err != nil {
			return nil, err
		}
		req.text = string(body)
		if strings.TrimSpace(req.text) == "" {
			return nil, httpErrf(http.StatusBadRequest, "empty %s body", mt)
		}
		return req, nil
	case "":
		return nil, httpErrf(http.StatusUnsupportedMediaType,
			"missing Content-Type: use %s, %s or %s", mtForm, mtSPARQLQuery, mtUpdate)
	default:
		return nil, httpErrf(http.StatusUnsupportedMediaType,
			"unsupported Content-Type %q: use %s, %s or %s", mt, mtForm, mtSPARQLQuery, mtUpdate)
	}
}

// protocolParams reads the operation text (unless the body carries it) and
// the dataset parameters from params.
func protocolParams(params url.Values, isUpdate bool) (*protocolRequest, error) {
	req := &protocolRequest{isUpdate: isUpdate}
	textKey, defKey, namedKey := "query", "default-graph-uri", "named-graph-uri"
	otherDef, otherNamed := "using-graph-uri", "using-named-graph-uri"
	if isUpdate {
		textKey, otherDef, defKey = "update", defKey, "using-graph-uri"
		otherNamed, namedKey = namedKey, "using-named-graph-uri"
	}
	if vs, ok := params[textKey]; ok {
		if len(vs) != 1 {
			return nil, httpErrf(http.StatusBadRequest, "exactly one %s= parameter is allowed, got %d", textKey, len(vs))
		}
		if strings.TrimSpace(vs[0]) == "" {
			return nil, httpErrf(http.StatusBadRequest, "empty %s= parameter", textKey)
		}
		if !utf8.ValidString(vs[0]) {
			return nil, httpErrf(http.StatusBadRequest, "%s= parameter is not valid UTF-8", textKey)
		}
		req.text = vs[0]
	}
	for _, k := range []string{otherDef, otherNamed} {
		if _, ok := params[k]; ok {
			return nil, httpErrf(http.StatusBadRequest, "%s= does not apply to a %s", k, map[bool]string{false: "query", true: "update"}[isUpdate])
		}
	}
	var err error
	if req.defaultGraphs, err = absoluteIRIs(params[defKey], defKey); err != nil {
		return nil, err
	}
	if req.namedGraphs, err = absoluteIRIs(params[namedKey], namedKey); err != nil {
		return nil, err
	}
	return req, nil
}

func absoluteIRIs(vs []string, key string) ([]string, error) {
	for _, v := range vs {
		if !iri.IsAbsolute(v) || !validIRIChars(v) {
			return nil, httpErrf(http.StatusBadRequest, "%s=%q is not an absolute IRI", key, v)
		}
	}
	return vs, nil
}

// validIRIChars rejects characters an IRIREF cannot contain.
func validIRIChars(s string) bool {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c <= ' ', c == '<', c == '>', c == '"', c == '{', c == '}', c == '|', c == '^', c == '`', c == '\\':
			return false
		}
	}
	return utf8.ValidString(s)
}

// readBody reads the request body up to limit bytes (no limit when limit <= 0).
func (x *exchange) readBody(limit int64) ([]byte, error) {
	body := x.bodyReader(limit)
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, bodyError(err, limit)
	}
	return data, nil
}

func (x *exchange) bodyReader(limit int64) io.Reader {
	if limit <= 0 {
		return x.r.Body
	}
	if x.r.ContentLength > limit {
		return errReader{&http.MaxBytesError{Limit: limit}}
	}
	return http.MaxBytesReader(x.w, x.r.Body, limit)
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func bodyError(err error, limit int64) error {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return httpErr(http.StatusRequestEntityTooLarge,
			fmt.Errorf("%w: the limit is %d bytes (raise it with WithMaxRequestBytes or WithMaxGraphBytes)", ErrBodyTooLarge, limit))
	}
	return httpErrf(http.StatusBadRequest, "reading the request body: %v", err)
}
