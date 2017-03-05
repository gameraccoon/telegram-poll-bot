package database

import (
	"testing"
	"os"
)

const (
	testDbPath = "./testDb.db"
)

func DropDatabase(fileName string) {
	os.Remove(fileName)
}

func CreateDbAndConnect(t *testing.T) *Database {
	DropDatabase(testDbPath)

	db := &Database{}

	err := db.Connect(testDbPath)
	if err != nil {
		t.Error("Problem with creation db connection:" + err.Error())
		return nil
	}
	return db
}

func TestConnection(t *testing.T) {
	DropDatabase(testDbPath)

	db := &Database{}

	if db.IsConnectionOpened() {
		t.Fail()
	}

	err := db.Connect(testDbPath)
		defer DropDatabase(testDbPath)
	if err != nil {
		t.Error("Problem with creation db connection:" + err.Error())
		return
	}

	if !db.IsConnectionOpened() {
		t.Fail()
	}

	db.Disconnect()

	if db.IsConnectionOpened() {
		t.Fail()
	}
}

func TestGetUserId(t *testing.T) {
	db := CreateDbAndConnect(t)
	defer DropDatabase(testDbPath)
	if db == nil {
		t.Fail()
		return
	}
	defer db.Disconnect()

	var chatId1 int64 = 321
	var chatId2 int64 = 123

	id1 := db.GetUserId(chatId1)
	id2 := db.GetUserId(chatId1)
	id3 := db.GetUserId(chatId2)

	if id1 != id2 {
		t.Fail()
	}

	if id1 == id3 {
		t.Fail()
	}
}

func TestCreateQuestion (t *testing.T) {
	db := CreateDbAndConnect(t)
	defer DropDatabase(testDbPath)
	if db == nil {
		t.Fail()
		return
	}
	defer db.Disconnect()

	userId := db.GetUserId(12)

	questionId := db.AddQuestion(userId, "Test question", 0, 5, 0)

	db.ActivateQuestion(questionId)

	readyUsers := db.GetReadyUsersChatIds()

	if len(readyUsers) != 1 {
		t.Fail()
	} else if readyUsers[0] != 12 {
		t.Fail()
	}
}

