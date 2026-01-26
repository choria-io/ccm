// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package modelmocks

import (
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/templates"
)

func NewManager(facts map[string]any, data map[string]any, noop bool, ctl *gomock.Controller) (*MockManager, *MockLogger) {
	logger := NewMockLogger(ctl)
	mgr := NewMockManager(ctl)

	var wd string

	mgr.EXPECT().Logger(gomock.Any()).AnyTimes().Return(logger, nil)
	mgr.EXPECT().UserLogger().AnyTimes().Return(logger)
	mgr.EXPECT().Facts(gomock.Any()).AnyTimes().Return(facts, nil)
	mgr.EXPECT().Data().AnyTimes().Return(data)
	mgr.EXPECT().NoopMode().AnyTimes().Return(noop)
	mgr.EXPECT().SetWorkingDirectory(gomock.Any()).DoAndReturn(func(d string) string { wd = d; return d }).AnyTimes()
	mgr.EXPECT().WorkingDirectory().DoAndReturn(func() string { return wd }).AnyTimes()
	mgr.EXPECT().TemplateEnvironment(gomock.Any()).AnyTimes().Return(&templates.Env{Facts: facts, Data: data}, nil)
	logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().With(gomock.Any()).AnyTimes().Return(logger)

	return mgr, logger
}
