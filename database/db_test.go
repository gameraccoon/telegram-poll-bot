package database

import (
	"testing"
	"github.com/stretchr/testify/require"
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
	assert := require.New(t)
	db := &Database{}

	err := db.Connect(testDbPath)
	if err != nil {
		assert.Fail("Problem with creation db connection:" + err.Error())
		return nil
	}
	return db
}

func createDbAndConnect(t *testing.T) *Database {
	clearDb()
	return connectDb(t)
}

func TestConnection(t *testing.T) {
	assert := require.New(t)
	dropDatabase(testDbPath)

	db := &Database{}

	assert.False(db.IsConnectionOpened())

	err := db.Connect(testDbPath)
	defer dropDatabase(testDbPath)
	if err != nil {
		assert.Fail("Problem with creation db connection:" + err.Error())
		return
	}

	assert.True(db.IsConnectionOpened())

	db.Disconnect()

	assert.False(db.IsConnectionOpened())
}

func TestGetUserId(t *testing.T) {
	assert := require.New(t)
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

	assert.Equal(id1, id2)
	assert.NotEqual(id1, id3)
}

func TestCreateQuestion(t *testing.T) {
	assert := require.New(t)
	//clearDb()

	var chatId int64 = 13

	{
		db := connectDb(t)
		userId := db.GetUserId(chatId)
		db.StartCreatingQuestion(userId)

		assert.True(db.IsUserEditingQuestion(userId))

		db.Disconnect()
	}

	{
		db := connectDb(t)
		userId := db.GetUserId(chatId)
		questionId := db.GetUserEditingQuestion(userId)

		assert.True(db.IsUserEditingQuestion(userId))
		assert.False(db.IsQuestionReady(questionId))
		db.SetQuestionText(questionId, "text")

		assert.Equal("text", db.GetQuestionText(questionId))
		assert.False(db.IsQuestionReady(questionId))

		db.Disconnect()
	}

	{
		db := connectDb(t)
		userId := db.GetUserId(chatId)
		questionId := db.GetUserEditingQuestion(userId)

		assert.Equal("text", db.GetQuestionText(questionId))
		assert.False(db.IsQuestionReady(questionId))

		db.SetQuestionVariants(questionId, []string{"v1", "v2"})

		assert.False(db.IsQuestionReady(questionId))
		assert.Equal(int64(2), db.GetQuestionVariantsCount(questionId))
		variants := db.GetQuestionVariants(questionId)
		assert.Equal(2, len(variants))
		if len(variants) == 2 {
			assert.Equal("v1", variants[0])
			assert.Equal("v2", variants[1])
		}

		db.Disconnect()
	}

	{
		db := connectDb(t)
		userId := db.GetUserId(chatId)
		questionId := db.GetUserEditingQuestion(userId)

		assert.False(db.IsQuestionReady(questionId))

		db.SetQuestionRules(questionId, 0, 5, 0)

		assert.True(db.IsQuestionReady(questionId))
		min, max, time := db.GetQuestionRules(questionId)
		assert.Equal(int64(0), min)
		assert.Equal(int64(5), max)
		assert.Equal(int64(0), time)

		db.Disconnect()
	}

	{
		db := connectDb(t)
		userId := db.GetUserId(chatId)
		questionId := db.GetUserEditingQuestion(userId)

		db.CommitQuestion(questionId)

		assert.False(db.IsUserEditingQuestion(userId))

		db.Disconnect()
	}
}

