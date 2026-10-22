package workbench

import (
	"encoding/json"

	"motty/internal/ai/store"
)

// The assistant's providers are shared between computers like the hosts are: the
// name, address and kind of each, which of its models are on, and that it has a
// key. The key itself is a saved password (aikey:<id>) and travels with the
// others. The assistant's own database stays what the assistant reads; the
// records are a copy kept in step with it, written when it changes and written
// back into it when a sync brings some in.

// savedAIProvider is a provider as a record.
type savedAIProvider struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Kind    string         `json:"kind"`
	BaseURL string         `json:"base_url,omitempty"`
	Enabled bool           `json:"enabled,omitempty"`
	HasKey  bool           `json:"has_key,omitempty"`
	Models  []savedAIModel `json:"models,omitempty"`
}

type savedAIModel struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled,omitempty"`
}

// savedAIProviders is the providers for the state, or nil while the assistant's
// database has not been read: a window that has not looked must not be taken to
// say there are none.
func (state *State) savedAIProviders() *[]savedAIProvider {
	ai := &state.ai
	if ai.store == nil || ai.storeErr != nil {
		return nil
	}
	list := make([]savedAIProvider, 0, len(ai.providers))
	for _, provider := range ai.providers {
		entry := savedAIProvider{
			ID: provider.ID, Name: provider.Name, Kind: provider.Kind, BaseURL: provider.BaseURL,
			Enabled: provider.Enabled, HasKey: provider.HasKey,
		}
		for _, model := range ai.models {
			if model.ProviderID == provider.ID {
				entry.Models = append(entry.Models, savedAIModel{Name: model.Name, Enabled: model.Enabled})
			}
		}
		list = append(list, entry)
	}
	return &list
}

// importAIProviders makes the assistant's database hold these providers: it adds
// and updates them, and drops the ones that are not in the list.
func (state *State) importAIProviders(list []savedAIProvider) {
	// Opening the database reads it, and a read saves; the records this window is
	// bringing in are not in its base yet, so that save would stamp them anew.
	known := state.recordedProviderIDs()
	state.pauseSave++
	db, err := state.aiStore()
	state.pauseSave--
	if err != nil || db == nil {
		return
	}
	current, err := db.Providers()
	if err != nil {
		return
	}
	wanted := make(map[string]bool, len(list))
	for _, entry := range list {
		if entry.ID == "" || (entry.Kind != store.KindOpenAI && entry.Kind != store.KindAnthropic) {
			continue
		}
		wanted[entry.ID] = true
		provider := store.Provider{ID: entry.ID, Name: entry.Name, Kind: entry.Kind, BaseURL: entry.BaseURL, HasKey: entry.HasKey, Enabled: entry.Enabled}
		if db.SaveProvider(provider) != nil {
			continue
		}
		models := make([]store.Model, 0, len(entry.Models))
		for _, model := range entry.Models {
			models = append(models, store.Model{ProviderID: entry.ID, Name: model.Name, Enabled: model.Enabled})
		}
		_ = db.SaveModels(entry.ID, models)
	}
	for _, provider := range current {
		// Only what the records once held can have been deleted elsewhere; a provider
		// here that was never recorded is new, not deleted.
		if known[provider.ID] && !wanted[provider.ID] {
			_ = db.DeleteProvider(provider.ID)
		}
	}
	state.pauseSave++
	state.reloadAI()
	state.pauseSave--
}

// adoptRecordedProviders restores providers from the records when the
// assistant's database has none: a database that was lost or set aside must not
// make the providers look deleted to every other computer.
func (state *State) adoptRecordedProviders(db *store.Store) {
	if state.configBase == nil {
		return
	}
	if current, err := db.Providers(); err != nil || len(current) > 0 {
		return
	}
	var recorded []savedAIProvider
	for key, payload := range state.configBase.payloads {
		if key.Kind != kindAIProvider {
			continue
		}
		var wrapper recordPayload
		var entry savedAIProvider
		if json.Unmarshal(payload, &wrapper) == nil && json.Unmarshal(wrapper.Data, &entry) == nil {
			recorded = append(recorded, entry)
		}
	}
	if len(recorded) == 0 {
		return
	}
	for _, entry := range recorded {
		if entry.Kind != store.KindOpenAI && entry.Kind != store.KindAnthropic {
			continue
		}
		_ = db.SaveProvider(store.Provider{ID: entry.ID, Name: entry.Name, Kind: entry.Kind, BaseURL: entry.BaseURL, HasKey: entry.HasKey, Enabled: entry.Enabled})
		models := make([]store.Model, 0, len(entry.Models))
		for _, model := range entry.Models {
			models = append(models, store.Model{ProviderID: entry.ID, Name: model.Name, Enabled: model.Enabled})
		}
		_ = db.SaveModels(entry.ID, models)
	}
}

const kindAIProvider = "ai-provider"

// recordedProviderIDs are the providers the records of this window's base hold.
func (state *State) recordedProviderIDs() map[string]bool {
	ids := map[string]bool{}
	if state.configBase != nil {
		for key := range state.configBase.payloads {
			if key.Kind == kindAIProvider {
				ids[key.ID] = true
			}
		}
	}
	return ids
}

func (state *State) recordedProviders() int { return len(state.recordedProviderIDs()) }
