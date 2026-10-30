package accounting

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// purgeCardSecrets removes the full card numbers, CVVs and provider tokens that cards used to keep, together
// with the keyed card-number fingerprints. The expiry is not sensitive on its own and moves to the card's
// details when the stored secret can still be decrypted. It runs at start-up and does nothing once clean.
func (e *Engine) purgeCardSecrets(ctx context.Context) {
	rows, err := e.db.QueryContext(ctx, `SELECT id::text,secret_ciphertext,COALESCE(details->>'expiry','') FROM accounting_vcc_cards
 WHERE secret_ciphertext <> '' OR card_fingerprint IS NOT NULL`)
	if err != nil {
		logger.LegacyPrintf("service.accounting", "Warning: card secret purge skipped: %v", err)
		return
	}
	type stored struct{ id, cipher, expiry string }
	var cards []stored
	for rows.Next() {
		var c stored
		if err = rows.Scan(&c.id, &c.cipher, &c.expiry); err != nil {
			_ = rows.Close()
			logger.LegacyPrintf("service.accounting", "Warning: card secret purge skipped: %v", err)
			return
		}
		cards = append(cards, c)
	}
	_ = rows.Close()
	purged := 0
	for _, c := range cards {
		expiry := c.expiry
		if expiry == "" && c.cipher != "" && e.encryptor != nil {
			if plain, decryptErr := e.encryptor.Decrypt(c.cipher); decryptErr == nil {
				secret := map[string]any{}
				if json.Unmarshal([]byte(plain), &secret) == nil {
					if value, _ := secret["expiry"].(string); value != "" {
						if _, parseErr := time.Parse("2006-01", value); parseErr == nil {
							expiry = value
						}
					}
				}
			}
		}
		// Secrets are cleared even when they cannot be decrypted: keeping them is the risk being removed.
		query := `UPDATE accounting_vcc_cards SET secret_ciphertext='',card_fingerprint=NULL WHERE id=$1`
		args := []any{c.id}
		if expiry != "" && expiry != c.expiry {
			query = `UPDATE accounting_vcc_cards SET secret_ciphertext='',card_fingerprint=NULL,details=jsonb_set(details,'{expiry}',to_jsonb($2::text)) WHERE id=$1`
			args = append(args, expiry)
		}
		if _, err = e.db.ExecContext(ctx, query, args...); err != nil {
			logger.LegacyPrintf("service.accounting", "Warning: card secret purge failed for card %s: %v", c.id, err)
			continue
		}
		purged++
	}
	if purged > 0 {
		logger.LegacyPrintf("service.accounting", "Removed stored card numbers and fingerprints from %d card(s)", purged)
	}
}
