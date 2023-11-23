// Copyright 2023 The KeepShare Authors. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"github.com/KeepShareOrg/keepshare/pkg/gormutil"
	"github.com/spf13/viper"
	"sync"
	"time"

	"github.com/KeepShareOrg/keepshare/config"
	"github.com/KeepShareOrg/keepshare/hosts"
	"github.com/KeepShareOrg/keepshare/hosts/pikpak/comm"
	lk "github.com/KeepShareOrg/keepshare/pkg/link"
	"github.com/KeepShareOrg/keepshare/pkg/share"
	"github.com/KeepShareOrg/keepshare/server/model"
	"github.com/KeepShareOrg/keepshare/server/query"
	log "github.com/sirupsen/logrus"
)

type AsyncBackgroundTask struct {
	concurrency     int
	unCompletedChan chan *model.SharedLink
}

func (a *AsyncBackgroundTask) PushAsyncTask(task *model.SharedLink) {
	a.unCompletedChan <- task
}

func (a *AsyncBackgroundTask) GetTaskFromDB() {
	getUncompletedToken := &GetUnCompletedToken{
		UpdatedTime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		OrderID:     0,
	}
	for {
		unCompleteTasks, token, err := getUnCompletedSharedLinks(cap(a.unCompletedChan), *getUncompletedToken)
		if token != nil {
			getUncompletedToken = token
		}
		if err != nil {
			log.Debugf("get uncompleted tasks err: %v", err)
		}
		if len(unCompleteTasks) == 0 {
			time.Sleep(time.Second * 2)
		}
		for _, task := range unCompleteTasks {
			a.PushAsyncTask(task)
		}
	}
}

func (a *AsyncBackgroundTask) taskConsumer() {
	for {
		unCompleteTask := <-a.unCompletedChan
		host := hosts.Get(unCompleteTask.Host)
		if host == nil {
			log.Errorf("host not found: %s", unCompleteTask.Host)
			continue
		}

		sharedLinks, err := host.CreateFromLinks(
			context.Background(),
			unCompleteTask.UserID,
			[]string{unCompleteTask.OriginalLink},
			unCompleteTask.CreatedBy,
		)
		if err != nil {
			log.Errorf("create share link error: %v", err.Error())
			continue
		}

		sh := sharedLinks[unCompleteTask.OriginalLink]

		if sh == nil {
			log.Errorf("link not found: %s", unCompleteTask.OriginalLink)
			continue
		}

		// if task processing duration grate than 48 hour, it's failed
		if sh.State == share.StatusCreated && time.Now().Sub(sh.CreatedAt).Hours() > 48 {
			if _, err := query.SharedLink.
				Where(query.SharedLink.AutoID.Eq(unCompleteTask.AutoID)).
				Update(query.SharedLink.State, share.StatusError); err != nil {
				log.Errorf("update share link error: %v", err.Error())
			}
			continue
		}

		if sh.State == share.StatusOK || sh.State == share.StatusCreated {
			now := time.Now()
			link := unCompleteTask.OriginalLink
			s := &model.SharedLink{
				AutoID:             unCompleteTask.AutoID,
				UserID:             unCompleteTask.UserID,
				State:              sh.State.String(),
				Host:               unCompleteTask.Host,
				CreatedBy:          sh.CreatedBy,
				CreatedAt:          unCompleteTask.CreatedAt,
				UpdatedAt:          now,
				Size:               sh.Size,
				Visitor:            sh.Visitor,
				Stored:             sh.Stored,
				Revenue:            sh.Revenue,
				Title:              sh.Title,
				OriginalLinkHash:   lk.Hash(link),
				HostSharedLinkHash: lk.Hash(sh.HostSharedLink),
				OriginalLink:       link,
				HostSharedLink:     sh.HostSharedLink,
			}

			if _, err = query.SharedLink.
				Where(query.SharedLink.AutoID.Eq(unCompleteTask.AutoID)).
				Updates(s); err != nil {
				log.Errorf("update share link state error: %v", err.Error())
			}
			continue
		}

		if _, err = query.SharedLink.
			Where(query.SharedLink.AutoID.Eq(unCompleteTask.AutoID)).
			Update(query.SharedLink.UpdatedAt, time.Now()); err != nil {
			log.Errorf("update share link updated_at error: %v", err.Error())
		}
	}
}

func (a *AsyncBackgroundTask) Run() {
	if a.concurrency <= 0 {
		a.concurrency = 16
	}

	go a.GetTaskFromDB()

	wg := sync.WaitGroup{}
	for i := 0; i < a.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.taskConsumer()
		}()
	}
	wg.Wait()
}

func NewAsyncBackgroundTask(concurrency int) *AsyncBackgroundTask {
	chSize := viper.GetInt("background_task_channel_size")
	if chSize <= 0 {
		chSize = 16 * 1024
	}

	return &AsyncBackgroundTask{
		concurrency:     concurrency,
		unCompletedChan: make(chan *model.SharedLink, chSize),
	}
}

var abt *AsyncBackgroundTask

func GetAsyncBackgroundTaskInstance() *AsyncBackgroundTask {
	if abt == nil {
		concurrency := viper.GetInt("background_task_concurrency")
		if concurrency <= 0 {
			concurrency = 16
		}
		abt = NewAsyncBackgroundTask(concurrency)
	}
	return abt
}

type GetUnCompletedToken struct {
	UpdatedTime time.Time
	OrderID     int64
}

// getUnCompletedSharedLinks get shared links that status in pending or created
func getUnCompletedSharedLinks(limitSize int, token GetUnCompletedToken) ([]*model.SharedLink, *GetUnCompletedToken, error) {
	s := query.SharedLink
	state := s.State.ColumnName().String()
	createdAt := s.CreatedAt.ColumnName().String()
	updatedAt := s.UpdatedAt.ColumnName().String()
	autoID := s.AutoID.ColumnName().String()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	unCompleteTasks := make([]*model.SharedLink, 0)

	w := fmt.Sprintf("`%s` in ('%s', '%s') AND (`%s`, `%s`) > ('%s', '%v') AND `%s`>'%s' AND TIMESTAMPDIFF(SECOND, `%s`, NOW())*60 > TIMESTAMPDIFF(SECOND, `%s`, `%s`)",
		state, string(share.StatusPending), string(share.StatusCreated),
		updatedAt, autoID, token.UpdatedTime.String(), token.OrderID,
		createdAt, time.Now().Add(-1*comm.RunningFilesMaxAge).Format(time.DateTime),
		updatedAt, createdAt, updatedAt,
	)
	err := config.MySQL().WithContext(gormutil.IgnoreTraceContext(ctx)).
		Where(w).
		Order(fmt.Sprintf("%v DESC", state)).
		Order(updatedAt).
		Order(autoID).
		Limit(limitSize).
		Find(&unCompleteTasks).
		Error

	if err != nil {
		return nil, nil, err
	}

	var nextToken *GetUnCompletedToken = nil
	if len(unCompleteTasks) > 0 {
		nextToken = &GetUnCompletedToken{
			UpdatedTime: unCompleteTasks[len(unCompleteTasks)-1].UpdatedAt,
			OrderID:     unCompleteTasks[len(unCompleteTasks)-1].AutoID,
		}
	}
	return unCompleteTasks, nextToken, nil
}
