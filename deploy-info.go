package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v2"
)

type ChartYaml struct {
	ApiVersion  string `yaml:"apiVersion"`
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"appVersion"`
	Icon        string `yaml:"icon"`
}

type ProjectsJson struct {
	ID                int           `json:"id"`
	Description       string        `json:"description"`
	Name              string        `json:"name"`
	NameWithNamespace string        `json:"name_with_namespace"`
	Path              string        `json:"path"`
	PathWithNamespace string        `json:"path_with_namespace"`
	CreatedAt         time.Time     `json:"created_at"`
	DefaultBranch     string        `json:"default_branch"`
	TagList           []interface{} `json:"tag_list"`
	SSHURLToRepo      string        `json:"ssh_url_to_repo"`
	HTTPURLToRepo     string        `json:"http_url_to_repo"`
	WebURL            string        `json:"web_url"`
	ReadmeURL         string        `json:"readme_url"`
	AvatarURL         interface{}   `json:"avatar_url"`
	StarCount         int           `json:"star_count"`
	ForksCount        int           `json:"forks_count"`
	LastActivityAt    time.Time     `json:"last_activity_at"`
	Namespace         struct {
		ID        int         `json:"id"`
		Name      string      `json:"name"`
		Path      string      `json:"path"`
		Kind      string      `json:"kind"`
		FullPath  string      `json:"full_path"`
		ParentID  int         `json:"parent_id"`
		AvatarURL interface{} `json:"avatar_url"`
		WebURL    string      `json:"web_url"`
	} `json:"namespace"`
}

type TagResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Target  string `json:"target"`
	Commit  struct {
		ID             string    `json:"id"`
		ShortID        string    `json:"short_id"`
		CreatedAt      time.Time `json:"created_at"`
		ParentIds      []string  `json:"parent_ids"`
		Title          string    `json:"title"`
		Message        string    `json:"message"`
		AuthorName     string    `json:"author_name"`
		AuthorEmail    string    `json:"author_email"`
		AuthoredDate   time.Time `json:"authored_date"`
		CommitterName  string    `json:"committer_name"`
		CommitterEmail string    `json:"committer_email"`
		CommittedDate  time.Time `json:"committed_date"`
	} `json:"commit"`
	Release interface{} `json:"release"`
}

type Compare struct {
	Commit struct {
		ID             string    `json:"id"`
		ShortID        string    `json:"short_id"`
		CreatedAt      time.Time `json:"created_at"`
		ParentIds      []string  `json:"parent_ids"`
		Title          string    `json:"title"`
		Message        string    `json:"message"`
		AuthorName     string    `json:"author_name"`
		AuthorEmail    string    `json:"author_email"`
		AuthoredDate   time.Time `json:"authored_date"`
		CommitterName  string    `json:"committer_name"`
		CommitterEmail string    `json:"committer_email"`
		CommittedDate  time.Time `json:"committed_date"`
	} `json:"commit"`
	Commits []struct {
		ID             string    `json:"id"`
		ShortID        string    `json:"short_id"`
		CreatedAt      time.Time `json:"created_at"`
		ParentIds      []string  `json:"parent_ids"`
		Title          string    `json:"title"`
		Message        string    `json:"message"`
		AuthorName     string    `json:"author_name"`
		AuthorEmail    string    `json:"author_email"`
		AuthoredDate   time.Time `json:"authored_date"`
		CommitterName  string    `json:"committer_name"`
		CommitterEmail string    `json:"committer_email"`
		CommittedDate  time.Time `json:"committed_date"`
	} `json:"commits"`
	Diffs []struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		AMode       string `json:"a_mode"`
		BMode       string `json:"b_mode"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
		Diff        string `json:"diff"`
	} `json:"diffs"`
	CompareTimeout bool `json:"compare_timeout"`
	CompareSameRef bool `json:"compare_same_ref"`
}

func main() {
	// command line args
	projectName := flag.String("p", "", "Gitlab Project Name")
	debug := flag.Bool("d", false, "print some debug output")
	appSourceBranch := flag.String("source", "master", "Application Source Branch. Default = master")
	cdTargetBranch := flag.String("target", "production", "Deployment Target Branch, Default = production")
	flag.Parse()

	// get env vars
	apiKey := os.Getenv("GITLAB_API_TOKEN")

	if *debug == true {
		fmt.Println("projectName:", *projectName)
		fmt.Println("apiKey:", apiKey)
		fmt.Println("appSourceBranch:", *appSourceBranch)
		fmt.Println("cdTargetBranch:", *cdTargetBranch)
	}

	// getProjectId for cd-projectName
	cdProjectName := fmt.Sprintf("cd-%v", *projectName)
	cdProjectId := getProjectId(apiKey, cdProjectName)
	if *debug == true {
		fmt.Println("cdProjectId:", cdProjectId)
	}
	// getProjectId for projectName
	appProjectId := getProjectId(apiKey, *projectName)
	if *debug == true {
		fmt.Println("appProjectId:", appProjectId)
	}
	// getVersion from cd-projectName of targetBranch
	deployedVersion := getVersion(apiKey, cdProjectId, *projectName, *cdTargetBranch)
	if *debug == true {
		fmt.Println("deployedVersion:", deployedVersion)
	}
	// getTag to discover git sha from projectName
	deployedTag := fmt.Sprintf("%v_v%v", *projectName, deployedVersion)
	deployedSha := getTag(apiKey, appProjectId, deployedTag)
	if *debug == true {
		fmt.Println("deployedSha:", deployedSha)
	}
	// compareBranches productionSha vs sourceBranch
	compare := compareBranches(apiKey, appProjectId, deployedSha, *appSourceBranch)
	if *debug == true {
		fmt.Println("compare:", compare)
		for i := 0; i < len(compare.Commits); i++ {
			fmt.Println(compare.Commits[i].Title)
		}
	}
	// attempt to discover JIRA tickets and insert hyperlinks in results
	// print Jira friendly output to terminal

	// *Commit Messages*
	// * list
	// * of
	// * messages
	// ---------------------
	// *Discovered JIRA Tickets*
	// * hyperlink https://mediciventures.atlassian.net/browse/ISSUE-ID

	fmt.Println("*Commit Messages*")
	for i := 0; i < len(compare.Commits); i++ {
		fmt.Printf("* %v\n", compare.Commits[i].Title)
	}
	fmt.Println("")
	fmt.Println("----------")
	fmt.Println("")
	fmt.Println("*JIRA Tickets*")
	r, regexpErr := regexp.Compile("(\\w*-\\d{1,})")
	if regexpErr != nil {
		log.Fatal(regexpErr)
	}
	for j := 0; j < len(compare.Commits); j++ {
		title := compare.Commits[j].Title
		match := r.MatchString(title)
		if match {
			ticket := r.FindString(title)
			fmt.Printf("[%v|https://mediciventures.atlassian.net/browse/%v]\n", ticket, ticket)
		}
	}
}

func getProjectId(apiKey string, projectName string) int {
	client := &http.Client{}
	uri := fmt.Sprintf("https://gitlab.com/api/v4/projects?simple=true&membership=true&search=%v", projectName)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("PRIVATE-TOKEN", apiKey)
	resp, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var projectsJson []ProjectsJson
	jsonErr := json.Unmarshal(body, &projectsJson)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	var projectId int
	for _, i := range projectsJson {
		if i.Name == projectName {
			projectId = i.ID
		} else {
			continue
		}
	}

	return projectId
}

func compareBranches(apiKey string, projectId int, from string, to string) Compare {
	client := &http.Client{}
	uri := fmt.Sprintf("https://gitlab.com/api/v4/projects/%v/repository/compare?from=%v&to=%v", projectId, from, to)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("PRIVATE-TOKEN", apiKey)
	resp, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var compare Compare
	jsonErr := json.Unmarshal(body, &compare)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	return compare
}

func getVersion(apiKey string, projectId int, projectName string, targetBranch string) string {
	client := &http.Client{}
	uri := fmt.Sprintf("https://gitlab.com/api/v4/projects/%v/repository/files/%v%%2FChart%%2Eyaml/raw?ref=%v", projectId, projectName, targetBranch)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("PRIVATE-TOKEN", apiKey)
	resp, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var ChartYaml ChartYaml
	yamlErr := yaml.Unmarshal(body, &ChartYaml)
	if yamlErr != nil {
		log.Fatal(yamlErr)
	}

	prodVersion := ChartYaml.Version
	return prodVersion
}

func getTag(apiKey string, projectId int, tag string) string {
	client := &http.Client{}
	uri := fmt.Sprintf("https://gitlab.com/api/v4/projects/%v/repository/tags?search=%v", projectId, tag)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("PRIVATE-TOKEN", apiKey)
	resp, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var tagResponse []TagResponse
	jsonErr := json.Unmarshal(body, &tagResponse)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	commitsha := tagResponse[0].Commit.ShortID
	return commitsha
}
