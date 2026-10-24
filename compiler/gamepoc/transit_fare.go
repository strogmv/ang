package gamepoc

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateTransitFares(c Compiled) error {
	var life struct {
		Edges []struct {
			ID      string `json:"id"`
			NameKey string `json:"name_key"`
			Mode    string `json:"mode"`
			Fare    *struct {
				Currency string `json:"currency"`
				Amount   int64  `json:"amount_minor"`
				Receiver string `json:"receiver_account"`
			} `json:"fare"`
		} `json:"edges"`
	}
	if len(c.Life) == 0 {
		return nil
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	keys := map[string]bool{}
	for _, m := range c.Localization.Messages {
		keys[m.Key] = true
	}
	for _, e := range life.Edges {
		if e.NameKey != "" && !keys[e.NameKey] {
			return fmt.Errorf("untranslated transit edge %s", e.ID)
		}
		if e.Fare == nil {
			continue
		}
		f := e.Fare
		a, ok := c.Inventory.Accounts[f.Receiver]
		currency, known := c.Inventory.Currencies[f.Currency]
		if e.NameKey == "" {
			return fmt.Errorf("paid transit needs a localized price label %s", e.ID)
		}
		for _, m := range c.Localization.Messages {
			if m.Key == e.NameKey {
				for locale, text := range m.Texts {
					if !strings.Contains(text, "{amount}") || !strings.Contains(text, "{currency}") {
						return fmt.Errorf("paid transit label %s/%s must expose amount and currency", e.ID, locale)
					}
				}
			}
		}
		if e.Mode != "rail" || f.Amount < 1 || f.Amount > 9000000000000000 || !ok || !known || currency.Carrier != "account" || a.Currency != f.Currency {
			return fmt.Errorf("invalid transit payment metadata %s", e.ID)
		}
	}
	return nil
}
