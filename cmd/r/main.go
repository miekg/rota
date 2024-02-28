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
}

var members = []rotang.Member{
	{Email: "miek.gieben@ru.nl"},
	{Email: "bram.daams@ru.nl"},
}

func main() {
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

	ss, err := g.Generate(sc, time.Now(), nil, members, 3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", ss)
}
