//go:build integration

package bi

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

func TestBIAdminResponsesMatchSupplementalOpenAPI(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/mobile-bi/admin.openapi.json")
	require.NoError(t, err)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	compiler := jsonschema.NewCompiler()
	const schemaURL = "https://schemas.devku.invalid/bi-admin.json"
	require.NoError(t, compiler.AddResource(schemaURL, document))
	s, c, p, _ := analysisFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers)
	r.GET("/:organization_id/grants", h.AdminListGrants)
	r.PUT("/:organization_id/grants", func(c *gin.Context) { h.AdminSaveGrant(c, p.UserID) })
	r.POST("/:organization_id/grants/:manager_id/revoke", func(c *gin.Context) { h.AdminRevokeGrant(c, p.UserID) })
	request := func(method, path, body string, status int, schema string) []byte {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/"+c.OrganizationID+path, strings.NewReader(body)))
		require.Equal(t, status, w.Code, w.Body.String())
		contract, err := compiler.Compile(schemaURL + "#/components/schemas/" + schema)
		require.NoError(t, err)
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(w.Body.Bytes()))
		require.NoError(t, err)
		require.NoError(t, contract.Validate(value), w.Body.String())
		return append([]byte{}, w.Body.Bytes()...)
	}
	var listed struct {
		Data struct{ Items []Grant }
	}
	require.NoError(t, json.Unmarshal(request("GET", "/grants", "", 200, "GrantListResponse"), &listed))
	require.Len(t, listed.Data.Items, 1)
	grant := listed.Data.Items[0]
	body, err := json.Marshal(GrantInput{UserID: p.UserID, Role: "viewer", AllTeams: true, TeamIDs: []string{}, Capabilities: []string{"analytics:read"}, ExpectedRevision: &grant.Revision})
	require.NoError(t, err)
	var saved struct{ Data Grant }
	require.NoError(t, json.Unmarshal(request("PUT", "/grants", string(body), 200, "GrantResponse"), &saved))
	require.Equal(t, grant.Revision+1, saved.Data.Revision)
	request("PUT", "/grants", string(body), 409, "Error")
	request("PUT", "/grants", `{`, 400, "Error")
	request("GET", "/grants?page=0", "", 400, "Error")
	body, err = json.Marshal(map[string]any{"expected_revision": saved.Data.Revision})
	require.NoError(t, err)
	request("POST", "/grants/"+p.ManagerID+"/revoke", string(body), 200, "RevokeGrantResponse")
}
