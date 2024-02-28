package main

import (
	"fmt"
	"log"
	"time"

	rotang "github.com/miekg/rota"
	"github.com/miekg/rota/pkg/algo"
)

var smembers = []rotang.ShiftMember{
	{Email: "miek.gieben@ru.nl", ShiftName: "postmaster"},
	{Email: "bram.daams@ru.nl", ShiftName: "postmaster"},
	{Email: "ben.polman@science.ru.nl", ShiftName: "postmaster"},
	{Email: "eric.liefers@science.ru.nl", ShiftName: "postmaster"},
	{Email: "simon.oosthoek@science.ru.nl", ShiftName: "postmaster"},
	{Email: "tobias.kunnen@science.ru.nl", ShiftName: "postmaster"},
	{Email: "wim.janssen@science.ru.nl", ShiftName: "postmaster"},
	{Email: "peter.vancampen@science.ru.nl", ShiftName: "postmaster"},
}

func main() {
	members := make([]rotang.Member, len(smembers))
	for i := range smembers {
		members[i] = rotang.Member{Email: smembers[i].Email}
	}
	members[0].Preferences = []rotang.Preference{rotang.NoOncall}

	gs := algo.New()
	gs.Register(algo.NewFair())
	gs.Register(algo.NewRandomGen())
	gs.RegisterModifier(algo.NewWeekendSkip())
	gs.RegisterModifier(algo.NewSplitShift())

	g, err := gs.Fetch("Fair")
	if err != nil {
		log.Fatal(err)
	}

	c := rotang.Config{
		Name:             "postmaster",
		ShiftsToSchedule: 2,
		Shifts: rotang.ShiftConfig{
			Generator:    "Fair",
			ShiftMembers: 2,
			Length:       7,
			Shifts: []rotang.Shift{
				{Name: "postmaster", Duration: 24 * 7 * time.Hour},
			},
		},
	}

	sc := &rotang.Configuration{
		Config:  c,
		Members: smembers,
	}

	ss, err := g.Generate(sc, time.Now(), nil, members, 5)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", ss)
}
