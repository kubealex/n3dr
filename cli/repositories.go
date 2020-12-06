package cli

import (
	"fmt"

	// "github.com/030/n3dr/cli/backup"

	validate "github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"
	"github.com/thedevsaddam/gojsonq"
)

type repositoriesNamesAndFormatsMap map[string]string

func (n *Nexus3) repositoriesNamesAndFormatsJSON() (string, error) {
	resp := n.request(n.URL + "/service/rest/" + n.APIVersion + "/repositories")
	err := resp.err
	if err != nil {
		return "", err
	}

	json := resp.strings
	if err := validate.New().Var(json, "required,json"); err != nil {
		return "", err
	}

	return json, nil
}

func repositoriesNamesAndFormatsJSONToMap(json string) (repositoriesNamesAndFormatsMap, error) {
	log.Debugf("JSON: '%v'", json)
	jq := gojsonq.New().JSONString(json).WhereNotEqual("type", "group")
	log.Debugf("JQ Output: '%v'", jq)
	jq.SortBy("name", "asc")
	nameAndFormat := jq.Only("name", "format")
	log.Debugf("NameAndFormat: '%v'", nameAndFormat)

	m := make(repositoriesNamesAndFormatsMap)
	for _, v := range nameAndFormat.([]interface{}) {
		if rec, ok := v.(map[string]interface{}); ok {
			m[rec["name"].(string)] = rec["format"].(string)
		} else {
			fmt.Println("FAIL")
		}
	}
	return m, nil
}

func (n *Nexus3) repositoriesNamesAndFormatsJSONToMapIncludingRequest() (repositoriesNamesAndFormatsMap, error) {
	json, err := n.repositoriesNamesAndFormatsJSON()
	if err != nil {
		return nil, err
	}

	m, err := repositoriesNamesAndFormatsJSONToMap(json)
	if err != nil {
		return nil, err
	}
	return m, err
}

func (n *Nexus3) RepositoryNames() error {
	m, err := n.repositoriesNamesAndFormatsJSONToMapIncludingRequest()
	if err != nil {
		return err
	}

	for name := range m {
		fmt.Printf("%s\n", name)
	}
	return nil
}

func (n *Nexus3) CountRepositories() error {
	log.Debug("Counting repositories...")
	m, err := n.repositoriesNamesAndFormatsJSONToMapIncludingRequest()
	if err != nil {
		return err
	}
	fmt.Println(len(m))
	return nil
}

func (n *Nexus3) repositoriesChannel(m repositoriesNamesAndFormatsMap, dir, regex string) error {
	log.Debugf("Repos: '%v'", m)
	errs := make(chan error)

	for name, format := range m {
		log.Debugf("Name: '%v'. Format: '%v'", name, format)

		go func(dir, format, name string) {
			switch format {
			case "maven2":
				n.Repository = name
				log.Debugf("Repository: '%v'", n.Repository)
				errs <- n.StoreArtifactsOnDiskChannel(dir, regex)
			case "npm":
				errs <- n.BackupAllNPMArtifacts(name, dir)
			default:
				errs <- fmt.Errorf("Nexus repository format: '%v' download not supported", format)
			}
		}(dir, format, name)
	}
	for range m {
		if err := <-errs; err != nil {
			return err
		}
	}
	return nil
}

func (n *Nexus3) downloadAllArtifactsFromRepositories(dir, regex string) error {
	m, err := n.repositoriesNamesAndFormatsJSONToMapIncludingRequest()
	if err != nil {
		return err
	}

	log.Debugf("Repositories: '%v'", m)
	if err := n.repositoriesChannel(m, dir, regex); err != nil {
		return err
	}
	return nil
}

// Downloads retrieves artifacts from all repositories
func (n *Nexus3) Downloads(regex string) error {
	dir, err := TempDownloadDir(n.DownloadDirName)
	if err != nil {
		return err
	}
	log.Info("Downloading artifacts from all repositories")
	if err := n.downloadAllArtifactsFromRepositories(dir, regex); err != nil {
		return err
	}
	if err := n.CreateZip(dir); err != nil {
		return err
	}
	return nil
}
