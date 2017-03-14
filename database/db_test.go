package database

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"os"
)

const (
	testDbPath = "./testDb.db"
)

func dropDatabase(fileName string) {
	os.Remove(fileName)
}

func clearDb() {
	dropDatabase(testDbPath)
}

func connectDb(t *testing.T) *Database {
	db := &Database{}

	err := db.Connect(testDbPath)
	if err != nil {
		assert.Fail(t, "Problem with creation db connection:" + err.Error())
		return nil
	}
	return db
}

func createDbAndConnect(t *testing.T) *Database {
	clearDb()
	return connectDb(t)
}

func TestConnection(t *testing.T) {
	dropDatabase(testDbPath)

	db := &Database{}

	assert.False(t, db.IsConnectionOpened())

	err := db.Connect(testDbPath)
	defer dropDatabase(testDbPath)
	if err != nil {
		assert.Fail(t, "Problem with creation db connection:" + err.Error())
		return
	}

	assert.True(t, db.IsConnectionOpened())

	db.Disconnect()

	assert.False(t, db.IsConnectionOpened())
}

func TestGetUserId(t *testing.T) {
	db := createDbAndConnect(t)
	defer clearDb()
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

	assert.Equal(t, id1, id2)
	assert.NotEqual(t, id1, id3)
}

func TestCreateQuestion (t *testing.T) {
	clearDb()
	defer clearDb()

	var chatId int64 = 12
	{
		db := connectDb(t)
		var userId int64 = db.GetUserId(chatId)
		db.StartCreatingQuestion(userId, "Test question", 0, 5, 0)
		db.Disconnect()
	}

	{
		db := connectDb(t)
		var userId int64 = db.GetUserId(chatId)
		db.CommitQuestion(userId)
		db.Disconnect()
	}

	db := connectDb(t)
	defer db.Disconnect()

	readyUsers := db.GetReadyUsersChatIds()

	assert.Equal(t, 0, len(readyUsers))
}

func TestReadyUser(t *testing.T) {
	clearDb()
	defer clearDb()
	db := connectDb(t)
	if db == nil {
		t.Fail()
		return
	}
	defer db.Disconnect()

	var chatId int64 = 12
	db.GetUserId(chatId)

	readyUsers := db.GetReadyUsersChatIds()
	assert.Equal(t, 1, len(readyUsers))
	if len(readyUsers) > 0 {
		assert.Equal(t, chatId, readyUsers[0])
	}

	db.SetUsersUnready([]int64{chatId})

	readyUsers2 := db.GetReadyUsersChatIds()
	assert.Equal(t, 0, len(readyUsers2))

	var userId2 int64 = db.GetUserId(chatId)

	readyUsers3 := db.GetReadyUsersChatIds()
	assert.Equal(t, 0, len(readyUsers3))

	db.SetUserReady(userId2)

	readyUsers4 := db.GetReadyUsersChatIds()
	assert.Equal(t, 1, len(readyUsers4))
	if len(readyUsers4) > 0 {
		assert.Equal(t, chatId, readyUsers4[0])
	}

	db.StartCreatingQuestion(userId2, "test", 0, 5, 0)
	db.SetVariants(userId2, []string{"v1", "v2"})

	readyUsers5 := db.GetReadyUsersChatIds()
	assert.Equal(t, 0, len(readyUsers5))

	db.CommitQuestion(userId2)

	readyUsers6 := db.GetReadyUsersChatIds()
	assert.Equal(t, 0, len(readyUsers6))

	db.AnswerNextQuestion(userId2, 0)

	readyUsers7 := db.GetReadyUsersChatIds()
	assert.Equal(t, 1, len(readyUsers7))
	if len(readyUsers7) > 0 {
		assert.Equal(t, chatId, readyUsers7[0])
	}
}

func TestAnswerQuestion(t *testing.T) {
	clearDb()
	//defer clearDb()

	var chatId1 int64 = 12
	var chatId2 int64 = 44

	// add users
	{
		db := connectDb(t)
		db.GetUserId(chatId1)
		db.GetUserId(chatId2)
		db.Disconnect()
	}

	// create question1
	{
		db := connectDb(t)
		var userId1 int64 = db.GetUserId(chatId1)
		db.StartCreatingQuestion(userId1, "Q1", 0, 2, 0)
		db.SetVariants(userId1, []string{"V1", "V2"})
		db.CommitQuestion(userId1)
		db.Disconnect()
	}

	// answer question1 by user2
	{
		db := connectDb(t)
		var userId2 int64 = db.GetUserId(chatId2)
		text, variants := db.GetNextQuestionForUser(userId2)
		assert.Equal(t, "Q1", text)
		assert.Equal(t, 2, len(variants))
		if len(variants) > 1 {
			assert.Equal(t, "V2", variants[1])
		}

		isNext, isEnded := db.AnswerNextQuestion(userId2, 1)

		assert.False(t, isNext)
		assert.False(t, isEnded)

		db.Disconnect()
	}

	// add question2
	{
		db := connectDb(t)
		var userId1 int64 = db.GetUserId(chatId1)
		db.StartCreatingQuestion(userId1, "Q2", 0, 1, 0)
		db.SetVariants(userId1, []string{"V3"})
		db.CommitQuestion(userId1)
		db.Disconnect()
	}

	// answer question1 by user1
	{
		db := connectDb(t)
		var userId1 int64 = db.GetUserId(chatId1)
		text, _ := db.GetNextQuestionForUser(userId1)
		assert.Equal(t, "Q1", text)

		isNext, isEnded := db.AnswerNextQuestion(userId1, 0)
		assert.True(t, isNext)
		assert.True(t, isEnded)

		db.Disconnect()
	}

	// answer question2 by user1
	{
		db := connectDb(t)
		var userId1 int64 = db.GetUserId(chatId1)
		text, variants := db.GetNextQuestionForUser(userId1)
		assert.Equal(t, "Q2", text)
		if len(variants) > 0 {
			assert.Equal(t, "V3", variants[0])
		}

		isNext, isEnded := db.AnswerNextQuestion(userId1, 0)

		assert.False(t, isNext)
		assert.True(t, isEnded)
	}
}

