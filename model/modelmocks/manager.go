// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package modelmocks

import (
	"github.com/choria-io/ccm/templates"
	"go.uber.org/mock/gomock"
)

func NewManager(facts map[string]any, data map[string]any, noop bool, ctl *gomock.Controller) (*MockManager, *MockLogger) {
	logger := NewMockLogger(ctl)
	mgr := NewMockManager(ctl)

	mgr.EXPECT().Logger(gomock.Any()).AnyTimes().Return(logger, nil)
	mgr.EXPECT().Facts(gomock.Any()).AnyTimes().Return(facts, nil)
	mgr.EXPECT().Data().AnyTimes().Return(data)
	mgr.EXPECT().NoopMode().AnyTimes().Return(noop)
	mgr.EXPECT().TemplateEnvironment(gomock.Any()).AnyTimes().Return(&templates.Env{Facts: facts, Data: data}, nil)
	logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

	return mgr, logger
}
