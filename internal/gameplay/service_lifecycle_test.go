package gameplay

import (
	"testing"
	"testing/synctest"
)

func TestGameServiceCloseJoinsAsynchronousNotifications(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := NewGameService()
		notified := make(chan GameSnapshot, 2)
		release := make(chan struct{})
		service.SetGameExpiredHandler(func(state GameSnapshot) {
			notified <- state
			<-release
		})
		service.notifyGameExpired(GameSnapshot{GameID: "game"})
		if state := <-notified; state.GameID != "game" {
			t.Fatalf("notification = %+v", state)
		}
		closed := make(chan struct{})
		go func() {
			service.Close()
			close(closed)
		}()
		synctest.Wait()
		select {
		case <-closed:
			t.Error("Close returned while a notification handler was still running")
		default:
		}
		close(release)
		<-closed
		service.notifyGameExpired(GameSnapshot{GameID: "after-close"})
		synctest.Wait()
		if len(notified) != 0 {
			t.Fatal("notification started after Close")
		}
	})
}
