// Copyright 2018 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package handlers

import (
	"infra/appengine/rotang"
	"infra/appengine/rotang/pkg/algo"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// State holds shared state between handlers.
type State struct {
	selfURL     string
	generators  *algo.Generators
	memberStore func(context.Context) rotang.MemberStorer
	shiftStore  func(context.Context) rotang.ShiftStorer
	configStore func(context.Context) rotang.ConfigStorer
	mailAddress string
	mailSender  rotang.MailSender
}

// Options contains the options used by the handlers.
type Options struct {
	URL         string
	Generators  *algo.Generators
	MailSender  rotang.MailSender
	MailAddress string

	MemberStore func(context.Context) rotang.MemberStorer
	ConfigStore func(context.Context) rotang.ConfigStorer
	ShiftStore  func(context.Context) rotang.ShiftStorer
}

// New creates a new handlers State container.
func New(opt *Options) (*State, error) {
	switch {
	case opt == nil:
		return nil, status.Errorf(codes.InvalidArgument, "opt can not be nil")
	case opt.URL == "":
		return nil, status.Errorf(codes.InvalidArgument, "URL must be set")
	case opt.Generators == nil:
		return nil, status.Errorf(codes.InvalidArgument, "Genarators can not be nil")
	case opt.MemberStore == nil, opt.ShiftStore == nil, opt.ConfigStore == nil:
		return nil, status.Errorf(codes.InvalidArgument, "Store functions can not be nil")
	}
	return &State{
		selfURL:     opt.URL,
		memberStore: opt.MemberStore,
		shiftStore:  opt.ShiftStore,
		configStore: opt.ConfigStore,
		mailSender:  opt.MailSender,
		mailAddress: opt.MailAddress,
	}, nil
}
