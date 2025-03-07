package service

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

type ModNexusService struct {
	apiKey     string
	baseUrl    string
	Client     *http.Client
	modMapping map[string]string // A mapping between mod nexus mod id's and the s3 prefix of mod files in hearthhub
}

type Mod struct {
	Id        string `json:"mod_id"`
	Version   string `json:"version"`
	Updated   string `json:"updated_time"`
	Created   string `json:"created_time"`
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Image     string `json:"picture_url"`
	Downloads int64  `json:"mod_downloads"`
	Author    string `json:"author"`
}

func MakeModNexusService() *ModNexusService {
	return &ModNexusService{
		apiKey:  os.Getenv("MOD_NEXUS_API_KEY"),
		baseUrl: "https://api.nexusmods.com",
		Client:  &http.Client{},
		modMapping: map[string]string{
			"4":    "mods/general/ValheimPlus.zip",
			"348":  "mods/general/BetterArchery.zip",
			"189":  "mods/general/BetterUI.zip",
			"92":   "mods/general/EquipmentAndQuickSlots.zip",
			"1042": "mods/general/PlantEverything.zip",
			"2323": "mods/general/ValheimPlus_Grant.zip",
		},
	}
}

func (m *ModNexusService) ModIdForPrefix(prefix string) string {
	return m.modMapping[prefix]

}

func (m *ModNexusService) GetMod(id string) (*Mod, error) {
	body, err := m.makeRequest(fmt.Sprintf("/v1/games/valheim/mods/%s.json", id))
	if err != nil {
		return nil, err
	}
	mod := new(Mod)
	err = json.Unmarshal(body, mod)

	if err != nil {
		return nil, err
	}
	return mod, nil
}

func (m *ModNexusService) makeRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("error while creating request to Mod Nexus: %v", err)
		return nil, err
	}
	req.Header.Add("apikey", m.apiKey)

	res, err := m.Client.Do(req)
	if err != nil {
		log.Errorf("error while making request to Mod Nexus: %v", err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Errorf("error while reading body of request to Mod Nexus: %v", err)
		return nil, err
	}

	return body, nil
}
