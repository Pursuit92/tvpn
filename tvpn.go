/*
 *  TVPN: A Peer-to-Peer VPN solution for traversing NAT firewalls
 *  Copyright (C) 2013  Joshua Chase <jcjoshuachase@gmail.com>
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License along
 *  with this program; if not, write to the Free Software Foundation, Inc.,
 *  51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
*/

package tvpn

import (
	"github.com/Pursuit92/LeveledLogger/log"
	"github.com/Pursuit92/state"
	"math/rand"
	"time"
)

type Friend struct {
	Validate bool
	Routes map[string]string
}

type TVPN struct {
	Friends     map[string]Friend
	Sig         SigBackend
	Stun        StunBackend
	VPN			VPNBackend
	States      map[string]*ConState
	Alloc		*IPManager
}

var rgen *rand.Rand
func init() {
	rgen = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func SetLogLevel(n int) {
	log.Out.SetLevel(n)
}

func SetLogPrefix(s string) {
	for i := 0; i < 10; i++ {
		log.Out.SetPrefix(i,s)
	}
}

func New(sig SigBackend, stun StunBackend, vpn VPNBackend, alloc *IPManager) *TVPN {
	tvpnInstance := TVPN{
		Sig:  sig,
		Stun: stun,
		VPN: vpn,
		Alloc: alloc,
		States: make(map[string]*ConState),
	}

	return &tvpnInstance
}

func (t *TVPN) Configure(conf Config) {
	t.Friends = conf.Friends
	t.Sig.Configure(conf.Sig)
	t.Stun.Configure(conf.Stun)
	t.VPN.Configure(conf.VPN)
	t.Alloc.Configure(conf.IPMan)
}

func (t TVPN) IsFriend(name string) (Friend,bool) {
	f,ok := t.Friends[name]
	return f,ok
}


func (t *TVPN) Run() error {
	var msg Message
	var err error

	t.Alloc.Init()

	err = t.Sig.Connect()

	if err != nil {
		log.Out.Lprintln(0,err)
		return err
	}

	for msg, err = t.Sig.RecvMessage(); err == nil; msg, err = t.Sig.RecvMessage() {
		switch msg.Type {
		case Init:
			log.Out.Lprintf(3,"Creating new state machine for %s\n",msg.From)
			st := NewState(msg.From,false,t)
			state.Input(st,msg)
			t.States[msg.From] = st
		case Join:
			log.Out.Lprintf(3,"Received Join from %s!\n",msg.From)
			_, ok := t.IsFriend(msg.From)
			if ok {
				st := NewState(msg.From,true,t)
				t.States[msg.From] = st
			}
			log.Out.Lprintln(3,"Done with join!")

		case Quit:
			st,exists := t.States[msg.From]
			if exists {
				st.Cleanup()
				delete(t.States,msg.From)
			}
		case Reset:
			t.States[msg.From].Reset(msg.Data["reason"])
		default:
			st,exists := t.States[msg.From]
			if exists {
				state.Input(st,msg)
			} else {
				// do stuff here
			}
		}
	}
	return err
}

func (t *TVPN) Start() {
	go t.Run()
}

func (t *TVPN) Stop() {
	t.Sig.Disconnect()
	t.Alloc.Stop()
	t.Cleanup()
}

func (t *TVPN) Cleanup() {
	for i,v := range t.States {
		v.Cleanup()
		delete(t.States, i)
	}
}

