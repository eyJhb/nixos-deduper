package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mcuadros/go-version"
)

var (
	// regexes to match for update package and init of package - (?i) for case insensitive
	regexUpdPkg  = regexp.MustCompile(`(?i)(?P<pkgname>[^:]+): (?P<fromver>.*) -> (?P<tover>[^ ]+)`)
	regexInitPkg = regexp.MustCompile(`(?i)(?P<pkgname>[^:]+): init at (?P<ver>[^ ]+)`)
	regexPrefix  = regexp.MustCompile(`(?i)^\[[^\]]+\]:?`)
	regexPrefix2 = regexp.MustCompile(`(?i)^(wip|somethingelse):?`)

	nixpkgsPath = "/home/eyjhb/projects/git/nixpkgs/"
)

type PullRequest struct {
	// information we get from GitHub API
	Title string `json:"title"`
	Id    int    `json:"id"`
	Url   string `json:"html_url"`
	Base  struct {
		Label string `json:"label"`
	} `json:"base"`

	// information we add ourself
	PkgInfo PackageUpdate
}

type PackageUpdate struct {
	Name        string
	FromVersion string
	ToVersion   string
}

func main() {
	// check if nixpkgsPath exists
	if _, err := os.Stat(nixpkgsPath); os.IsNotExist(err) {
		fmt.Printf("The specified nixpkgs path does not exists '%s'\n", nixpkgsPath)
		return
	}

	prs, _ := getPRsList()
	pkgList := make(map[string][]*PullRequest)
	for key := range prs {
		pr := prs[key]
		pr.Title = fixupName(pr.Title)

		// exclude everything that isn't based against master
		// sometimes things will get backported etc...
		if pr.Base.Label != "NixOS:master" {
			continue
		}

		if exs, err := checkPkgUpdate(pr.Title); err == nil {
			pr.PkgInfo = exs
		} else if exs, err := checkPkgInit(pr.Title); err == nil {
			pr.PkgInfo = exs
		} else {
			continue
		}

		pkgList[pr.PkgInfo.Name] = append(pkgList[pr.PkgInfo.Name], &pr)
	}

	var printedName bool
	for name, list := range pkgList {
		printedName = false

		for _, item := range list {
			var prefix string
			if item.PkgInfo.FromVersion != "" && item.PkgInfo.ToVersion != "" {
				prefix = "upda"
			} else if item.PkgInfo.FromVersion == "" && item.PkgInfo.ToVersion != "" {
				prefix = "init"
			}

			localVersion, bigger := compareVersions(item.PkgInfo.Name, item.PkgInfo.ToVersion)
			if len(list) > 1 || bigger {
				if !printedName {
					fmt.Printf("%s\n", name)
					printedName = true
				}

				fmt.Printf("- %s | %s -> %s ( %s )\n", prefix, item.PkgInfo.FromVersion, item.PkgInfo.ToVersion, item.Url)
				fmt.Printf("-- nixPkgs >= PRVersion | %s >= %s\n", localVersion, item.PkgInfo.ToVersion)
			}
			// // this is a update request
			// if item.PkgInfo.FromVersion != "" && item.PkgInfo.ToVersion != "" {
			// 	// if the

			// } else if item.PkgInfo.FromVersion == "" && item.PkgInfo.ToVersion != "" {
			// 	localVersion, bigger := compareVersions(item.PkgInfo.Name, item.PkgInfo.ToVersion)
			// 	if len(list) > 1 || bigger {
			// 		fmt.Printf("- upda | %s -> %s ( %s )\n", item.PkgInfo.FromVersion, item.PkgInfo.ToVersion, item.Url)
			// 		fmt.Printf("-- localVersion >= PRVersion | %s >= %s\n", localVersion, item.PkgInfo.ToVersion)
			// 	}
			// }
			// prefix = "init"
			// }
			// if prefix != "init" || (prefix == "init" && len(list) > 1) {
			// fmt.Printf("- %s | %s -> %s ( %s )\n", prefix, item.PkgInfo.FromVersion, item.PkgInfo.ToVersion, item.Url)
			// }

		}
	}

}

func compareVersions(name, ver string) (string, bool) {
	if localVersion := getPackageVersion(name); localVersion != "" {
		if version.Compare(localVersion, ver, ">=") {
			return localVersion, true
		}
	}

	return "UNKN", false
}

func getPackageVersion(pkgName string) string {
	// build our arguments
	cmdArgs := []string{"eval"}
	if nixpkgsPath != "" {
		cmdArgs = append(cmdArgs, []string{"-I", fmt.Sprintf("nixpkgs=%s", nixpkgsPath)}...)
	}
	cmdArgs = append(cmdArgs, []string{"--raw", fmt.Sprintf("nixpkgs.%s.version", pkgName)}...)

	// execute command
	cmd := exec.Command("nix", cmdArgs...)
	// fmt.Println(cmdArgs)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

func fixupName(name string) string {
	name = regexPrefix.ReplaceAllString(name, "")
	name = regexPrefix2.ReplaceAllString(name, "")

	name = strings.TrimSpace(name)

	return name
}

func checkPkgInit(text string) (PackageUpdate, error) {
	valueMap, err := subexpSearch(text, regexInitPkg)
	if err != nil {
		return PackageUpdate{}, err
	}

	return PackageUpdate{
		Name:      valueMap["pkgname"],
		ToVersion: valueMap["ver"],
	}, nil
}

func checkPkgUpdate(text string) (PackageUpdate, error) {
	valueMap, err := subexpSearch(text, regexUpdPkg)
	if err != nil {
		return PackageUpdate{}, err
	}

	return PackageUpdate{
		Name:        valueMap["pkgname"],
		FromVersion: valueMap["fromver"],
		ToVersion:   valueMap["tover"],
	}, nil
}

func subexpSearch(text string, exp *regexp.Regexp) (map[string]string, error) {
	// make our map
	valueMap := make(map[string]string)

	values := exp.FindStringSubmatch(text)
	keys := exp.SubexpNames()

	if len(values) == 0 {
		return valueMap, errors.New("Could not extract info")
	}

	// map found values to the capture groups in the regex
	// if capture group name is "", then skip it
	for i, k := range keys {
		if k == "" {
			continue
		}

		valueMap[k] = values[i]
	}

	return valueMap, nil
}

// read PRs from our files
func getPRsList() ([]PullRequest, error) {
	prs := []PullRequest{}

	// read files in output
	files, err := ioutil.ReadDir("outputs/")
	if err != nil {
		return prs, err
	}

	// loop each file
	for _, file := range files {
		// we do not want dirs, could probably
		// also check prefix of files instead..
		if file.IsDir() || file.Name() == ".gitkeep" {
			continue
		}

		// read our file
		bs, err := ioutil.ReadFile(fmt.Sprintf("outputs/%s", file.Name()))
		if err != nil {
			return prs, err
		}

		// tmp variable to hold information
		var tmp []PullRequest
		err = json.Unmarshal(bs, &tmp)
		if err != nil {
			return prs, err
		}

		// append it to our final prs
		prs = append(prs, tmp...)

	}

	//return it
	return prs, nil
}
