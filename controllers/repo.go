package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/orm"
	"io/ioutil"
	"issue_pr_board/models"
	"issue_pr_board/utils"
	"net/http"
	"os"
	"strings"
)

type ReposController struct {
	BaseController
}

type QueryRepoParam struct {
	Keyword   string
	Sig       string
	Page      int
	PerPage   int
	Direction string
}

func formQueryRepoSql(q QueryRepoParam) (int64, string) {
	rawSql := "select * from repo"
	keyword := q.Keyword
	sig := q.Sig
	page := q.Page
	perPage := q.PerPage
	direction := q.Direction
	if keyword != "" {
		if len(rawSql) == 18 {
			rawSql += fmt.Sprintf(" where instr (name, '%s')", strings.ToLower(keyword))
		} else {
			rawSql += fmt.Sprintf(" where instr (name, '%s')", strings.ToLower(keyword))
		}
	}
	if sig != "" {
		if len(rawSql) == 18 {
			rawSql += fmt.Sprintf(" where sig='%s'", sig)
		} else {
			rawSql += fmt.Sprintf(" and sig='%s'", sig)
		}
	}
	if direction != "desc" {
		rawSql += " order by name"
	} else {
		rawSql += " order by name desc"
	}
	var repo []models.Repo
	o := orm.NewOrm()
	count, err := o.Raw(rawSql).QueryRows(&repo)
	if err != nil {
		return 0, "select * from repo"
	}
	offset := perPage * (page - 1)
	rawSql += fmt.Sprintf(" limit %v offset %v", perPage, offset)
	return count, rawSql
}

func (c *ReposController) Get() {
	var repo []models.Repo
	page, _ := c.GetInt("page", 1)
	perPage, _ := c.GetInt("per_page", 10)
	qp := QueryRepoParam{
		Keyword:   c.GetString("keyword", ""),
		Sig:       c.GetString("sig", ""),
		Page:      page,
		PerPage:   perPage,
		Direction: c.GetString("direction", ""),
	}
	count, sql := formQueryRepoSql(qp)
	o := orm.NewOrm()
	_, err := o.Raw(sql).QueryRows(&repo)
	if err == nil {
		c.ApiDataReturn(count, page, perPage, repo)
	}
}

func SearchRepo(name string) bool {
	o := orm.NewOrm()
	searchSql := fmt.Sprintf("select * from repo where name='%s'", name)
	err := o.Raw(searchSql).QueryRow()
	if err == orm.ErrNoRows {
		return false
	}
	return true
}

type RepoResponse struct {
	Id        int    `json:"id"`
	FullName  string `json:"full_name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func SyncRepoNumber() error {
	logs.Info("Starting to sync repos numbers...")
	page := 1
	for {
		logs.Info("Sync repos: Page", page)
		url := fmt.Sprintf("https://gitee.com/api/v5/enterprises/open_euler/repos?type=all&page=%v&per_page=100&access_token=%v", page, os.Getenv("AccessToken"))
		resp, err := http.Get(url)
		if err != nil {
			logs.Error("Fail to get enterprise pull requests, err：", err)
			return err
		}
		if resp.StatusCode != 200 {
			logs.Error("Get unexpected response when getting V8 enterprise repos, status:", resp.Status)
			continue
		}
		body, _ := ioutil.ReadAll(resp.Body)
		err = resp.Body.Close()
		if err != nil {
			logs.Error("Fail to close response body of V8 enterprise repos, err：", err)
			return err
		}
		if len(string(body)) == 2 {
			break
		}
		var repos []RepoResponse
		err = json.Unmarshal(body, &repos)
		if err != nil {
			logs.Error(err)
			return nil
		}
		if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			var r models.Repo
			name := repo.FullName
			number := repo.Id
			createdAt := repo.CreatedAt
			updatedAt := repo.UpdatedAt
			r.Name = name
			r.EnterpriseNumber = number
			r.CreatedAt = utils.FormatTime(createdAt)
			r.UpdatedAt = utils.FormatTime(updatedAt)
			if SearchRepo(name) {
				o := orm.NewOrm()
				qs := o.QueryTable("repo")
				_, err := qs.Filter("name", name).Update(orm.Params{
					"enterprise_number": number,
					"created_at":        r.CreatedAt,
					"updated_at":        r.UpdatedAt,
				})
				if err != nil {
					logs.Error("Update repo enterprise number failed, err:", err)
				} else {
					logs.Info("更新仓库", name)
				}
			}
		}
		page += 1
	}
	logs.Info("Ends of repos numbers sync, wait the next time...")
	return nil
}
