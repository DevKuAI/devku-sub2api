//go:build integration

package bi

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConcurrentGrantEditorsUseConsistentManagerLockOrder(t *testing.T) {
	s, _, users := authFixture(t)
	var groupID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO groups(name) VALUES($1) RETURNING id`, randomToken("group_")).Scan(&groupID))
	orgs := []string{}
	for _, userID := range users {
		id := fmt.Sprintf("grant_org_%d", userID)
		orgs = append(orgs, id)
		_, err := integrationDB.Exec(`INSERT INTO desktop_organizations(public_id,code,name,gateway_user_id,group_id) VALUES($1,$2,'Grant test',$3,$4)`, id, fmt.Sprintf("g%d", userID), userID, groupID)
		require.NoError(t, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for round := int64(0); round < 20; round++ {
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for index := 0; index < 2; index++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				_, err := s.SaveGrant(ctx, orgs[index], GrantInput{UserID: users[index], Role: "viewer", AllTeams: true, TeamIDs: []string{}, Capabilities: []string{"analytics:read"}, ExpectedRevision: &round}, users[1-index], "concurrent-edit")
				results <- err
			}(index)
		}
		close(start)
		wg.Wait()
		close(results)
		for err := range results {
			require.NoError(t, err)
		}
	}
}
