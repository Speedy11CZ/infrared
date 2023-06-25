package infrared_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

func TestCPN_ListenAndServe(t *testing.T) {
	ctrl := gomock.NewController(t)
	tt := []struct {
		name    string
		err     error
		in      *MockConn
		out     *MockPlayer
		procDur time.Duration
		procErr error
	}{
		{
			name:    "ProcessConn",
			in:      mockConn(ctrl),
			out:     mockPlayer(ctrl),
			procDur: time.Millisecond,
		},
		{
			name:    "ProcessConn_ConnTimesOut",
			in:      mockConn(ctrl),
			procDur: time.Millisecond * 2,
			procErr: errors.New(""),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cp := NewMockConnProcessor(ctrl)
			cp.EXPECT().ClientTimeout().Times(1).Return(time.Duration(0))
			cp.EXPECT().ProcessConn(tc.in).Times(1).Return(tc.out, tc.procErr)
			replyChan := make(chan event.Reply)
			close(replyChan)
			bus := NewMockBus(ctrl)
			bus.EXPECT().Request(gomock.Any(), infrared.PreProcessingEventTopic).
				Times(1).Return(replyChan)

			if tc.err == nil {
				tc.in.EXPECT().SetDeadline(gomock.Any()).Times(1).Return(nil)
			}

			if tc.out == nil {
				tc.in.EXPECT().Close().Times(1).Return(nil)
			} else {
				tc.in.EXPECT().SetDeadline(time.Time{}).Times(1).Return(nil)
				bus.EXPECT().Request(gomock.Any(), infrared.PostProcessingEventTopic).
					Times(1).Return(replyChan)
			}

			in := make(chan infrared.Conn)
			out := make(chan infrared.Player, 1)
			cpn := infrared.CPN{
				ConnProcessor: cp,
				In:            in,
				Out:           out,
				Logger:        zap.NewNop(),
				EventBus:      bus,
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			quit := make(chan struct{})
			go func() {
				cpn.ListenAndServe(quit)
				wg.Done()
			}()
			in <- tc.in
			quit <- struct{}{}

			if tc.out != nil {
				if <-out != tc.out {
					t.Fail()
				}
			}

			wg.Wait()
		})
	}
}
