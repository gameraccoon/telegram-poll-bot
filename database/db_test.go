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

	assert.Equal(chatId1, db.GetUserChatId(id1))
	assert.Equal(chatId2, db.GetUserChatId(id3))
}

func TestUserReady(t *testing.T) {
	assert := require.New(t)
	db := createDbAndConnect(t)
	defer clearDb()
	if db == nil {
		t.Fail()
		return
	}
	defer db.Disconnect()

	var chatId int64 = 3221
	userId := db.GetUserId(chatId)

	{
		readyUsers := db.GetReadyUsersChatIds()
		assert.Equal(1, len(readyUsers))
		assert.Equal(chatId, readyUsers[0])
	}

	db.UnmarkUserReady(userId)

	{
		readyUsers := db.GetReadyUsersChatIds()
		assert.Equal(0, len(readyUsers))
	}

	db.MarkUserReady(userId)

	{
		readyUsers := db.GetReadyUsersChatIds()
		assert.Equal(1, len(readyUsers))
		assert.Equal(chatId, readyUsers[0])
	}
}

func TestCreateQuestion(t *testing.T) {
	assert := require.New(t)
	clearDb()
	defer clearDb()

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
		assert.Equal("v1", variants[0])
		assert.Equal("v2", variants[1])

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

func TestDiscardQustion(t *testing.T) {
	assert := require.New(t)
	clearDb()
	defer clearDb()

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

		db.DiscardQuestion(questionId)

		assert.False(db.IsUserEditingQuestion(userId))
		db.Disconnect()
	}
}

func TestAnswerQuestion(t *testing.T) {
	assert := require.New(t)
	clearDb()
	defer clearDb()

	var chatId1 int64 = 13
	var chatId2 int64 = 95
	var chatId3 int64 = 45

	{
		db := connectDb(t)
		userId1 := db.GetUserId(chatId1)
		userId2 := db.GetUserId(chatId2)
		userId3 := db.GetUserId(chatId3)
		db.StartCreatingQuestion(userId1)
		db.UnmarkUserReady(userId1)
		questionId := db.GetUserEditingQuestion(userId1)
		db.SetQuestionText(questionId, "text")
		db.SetQuestionVariants(questionId, []string{"v1", "v2", "v3"})
		db.SetQuestionRules(questionId, 0, 2, 0)
		db.CommitQuestion(questionId)
		db.MarkUserReady(userId1)

		assert.True(db.IsUserHasPendingQuestions(userId1))
		assert.True(db.IsUserHasPendingQuestions(userId2))
		assert.True(db.IsUserHasPendingQuestions(userId3))

		readyUsers := db.GetReadyUsersChatIds()

		assert.Equal(3, len(readyUsers))
		// order can be changed
		assert.Equal(int64(13), readyUsers[0])
		assert.Equal(int64(95), readyUsers[1])
		assert.Equal(int64(45), readyUsers[2])

		db.Disconnect()
	}

	{
		db := connectDb(t)
		userId1 := db.GetUserId(chatId1)

		assert.True(db.IsUserHasPendingQuestions(userId1))

		questionId := db.GetUserNextQuestion(userId1)
		db.AddQuestionAnswer(questionId, userId1, int64(0))
		db.RemoveUserPendingQuestion(userId1, questionId)
		db.Disconnect()
	}

	{
		db := connectDb(t)
		userId2 := db.GetUserId(chatId2)

		assert.True(db.IsUserHasPendingQuestions(userId2))

		questionId := db.GetUserNextQuestion(userId2)
		db.AddQuestionAnswer(questionId, userId2, int64(1))
		db.RemoveUserPendingQuestion(userId2, questionId)
		db.EndQuestion(questionId)
		users := db.GetUsersAnsweringQuestionNow(questionId)
		assert.Equal(1, len(users))

		for _, user := range(users) {
			db.RemoveUserPendingQuestion(user, questionId)

			assert.False(db.IsUserEditingQuestion(user))

			if !db.IsUserHasPendingQuestions(user) {
				db.MarkUserReady(user)
			}

		}
		db.RemoveQuestionFromAllUsers(questionId)

		respondents := db.GetQuestionRespondents(questionId)

		assert.Equal(2, len(respondents))
		// order can be changed
		assert.Equal(int64(13), respondents[0])
		assert.Equal(int64(95), respondents[1])

		answers := db.GetQuestionAnswers(questionId)

		assert.Equal(3, len(answers))
		assert.Equal(int64(1), answers[0])
		assert.Equal(int64(1), answers[1])
		assert.Equal(int64(0), answers[2])

		db.Disconnect()
	}
}

