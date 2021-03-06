package database

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strconv"
	"strings"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type Database struct {
	// connection
	conn *sql.DB
}

func sanitizeString(input string) (result string) {
	result = input
	result = strings.Replace(result, "'", "''", -1)
	return
}

func (database *Database) execQuery(query string) {
	_, err := database.conn.Exec(query)

	if err != nil {
		log.Fatal(err.Error())
	}
}

func (database *Database) Connect(fileName string) error {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		log.Fatal(err.Error())
		return err
	}

	database.conn = db

	database.execQuery("PRAGMA foreign_keys = ON")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" global_vars(name TEXT PRIMARY KEY" +
		",integer_value INTEGER" +
		",string_value STRING);")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" users(id INTEGER NOT NULL PRIMARY KEY" +
		",chat_id INTEGER UNIQUE NOT NULL" +
		",is_ready INTEGER NOT NULL" +
		",banned INTEGER" +
		")")

	database.execQuery("CREATE UNIQUE INDEX IF NOT EXISTS" +
		" chat_id_index ON users(chat_id)")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" questions(id INTEGER NOT NULL PRIMARY KEY" +
		",author INTEGER" +
		",text STRING" +
		",status INTEGER NOT NULL" + // 0 - editing, 1 - opened, 2 - closed
		",min_votes INTEGER" +
		",max_votes INTEGER" +
		",end_time INTEGER" +
		",FOREIGN KEY(author) REFERENCES users(id) ON DELETE SET NULL" +
		")")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" variants(id INTEGER NOT NULL PRIMARY KEY" +
		",question_id INTEGER NOT NULL" +
		",text STRING NOT NULL" +
		",votes_count INTEGER NOT NULL" +
		",index_number INTEGER NOT NULL" +
		",FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE" +
		")")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" answered_questions(id INTEGER NOT NULL PRIMARY KEY" +
		",user_id INTEGER NOT NULL" +
		",question_id INTEGER NOT NULL" +
		",FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE" +
		",FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE" +
		")")

	database.execQuery("CREATE TABLE IF NOT EXISTS" +
		" pending_questions(id INTEGER NOT NULL PRIMARY KEY" +
		",user_id INTEGER NOT NULL" +
		",question_id INTEGER NOT NULL" +
		",FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE" +
		",FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE" +
		")")

	return nil
}

func (database *Database) Disconnect() {
	database.conn.Close()
	database.conn = nil
}

func (database *Database) IsConnectionOpened() bool {
	return database.conn != nil
}

func (database *Database) createUniqueRecord(table string, values string) int64 {
	var err error
	if len(values) == 0 {
		_, err = database.conn.Exec(fmt.Sprintf("INSERT INTO %s DEFAULT VALUES ", table))
	} else {
		_, err = database.conn.Exec(fmt.Sprintf("INSERT INTO %s VALUES (%s)", table, values))
	}

	if err != nil {
		log.Fatal(err.Error())
		return -1
	}

	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM %s ORDER BY id DESC LIMIT 1", table))

	if err != nil {
		log.Fatal(err.Error())
		return -1
	}
	defer rows.Close()

	if rows.Next() {
		var id int64
		err := rows.Scan(&id)
		if err != nil {
			log.Fatal(err.Error())
			return -1
		}

		return id
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal("No record created")
	return -1
}

func (database *Database) GetUserId(chatId int64) (userId int64) {
	database.execQuery(fmt.Sprintf("INSERT OR IGNORE INTO users(chat_id, is_ready) "+
		"VALUES (%d, 1)", chatId))

	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM users WHERE chat_id=%d", chatId))
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&userId)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No user found")
	}

	return
}

func (database *Database) GetUserChatId(userId int64) (chatId int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT chat_id FROM users WHERE id=%d", userId))
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&chatId)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No user found")
	}

	return
}

func (database *Database) GetUserEditingQuestion(userId int64) (questionId int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM questions WHERE status=0 AND author=%d", userId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&questionId)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No question found")
	}

	return
}

func (database *Database) GetUserNextQuestion(userId int64) (questionId int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT MIN(question_id) FROM pending_questions WHERE user_id=%d", userId))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&questionId)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No question found")
	}

	return
}

func (database *Database) IsUserEditingQuestion(userId int64) bool {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM questions WHERE status=0 AND author=%d", userId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		count := 0
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
			return false
		}

		if count != 0 {
			if count != 1 {
				log.Fatalf("Count should be 0 or 1: %d", count)
			}
			return true
		}
		return false
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
			return false
		}
		log.Fatal("No question found")
		return false
	}
}

func (database *Database) IsUserHasPendingQuestions(userId int64) bool {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM pending_questions WHERE user_id=%d", userId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		count := 0
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
			return false
		}

		return (count > 0)
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
			return false
		}
		log.Fatal("No question found")
		return false
	}
}

func (database *Database) GetQuestionText(questionId int64) (text string) {
	text = ""
	rows, err := database.conn.Query(fmt.Sprintf("SELECT text FROM questions WHERE id=%d", questionId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&text)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No question found")
	}

	return
}

func (database *Database) GetQuestionVariants(questionId int64) (variants []string) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT text FROM variants WHERE question_id=%d", questionId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var variant string
		err := rows.Scan(&variant)
		if err != nil {
			log.Fatal(err.Error())
		}
		variants = append(variants, variant)
	}

	return
}

func (database *Database) GetQuestionVariantsCount(questionId int64) (count int) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM variants WHERE question_id=%d", questionId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No question found")
	}

	return
}

func (database *Database) GetQuestionRules(questionId int64) (minAnswers int, maxAnswers int, endTime int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT min_votes,max_votes,end_time FROM questions WHERE id=%d", questionId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&minAnswers, &maxAnswers, &endTime)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No question found")
	}

	return

}

func (database *Database) GetQuestionAnswers(questionId int64) (answers []int) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT votes_count FROM variants WHERE question_id=%d ORDER BY index_number ASC", questionId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var answer int
		err := rows.Scan(&answer)
		if err != nil {
			log.Fatal(err.Error())
		}
		answers = append(answers, answer)
	}

	return
}

func (database *Database) GetQuestionAnswersCount(questionId int64) (count int) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM answered_questions WHERE question_id=%d", questionId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No question found")
	}

	return
}

func (database *Database) SetQuestionRules(questionId int64, minVotes int, maxVotes int, time int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK questions SET"+
		" min_votes=%d"+
		",max_votes=%d"+
		",end_time=%d"+
		" WHERE id=%d", minVotes, maxVotes, time, questionId))
}

func (database *Database) SetQuestionText(questionId int64, text string) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK questions SET"+
		" text='%s'"+
		" WHERE id=%d", sanitizeString(text), questionId))
}

func (database *Database) SetQuestionVariants(questionId int64, variants []string) {
	// delete the old variants
	database.execQuery(fmt.Sprintf("DELETE FROM variants WHERE question_id=%d", questionId))

	// add the new ones
	var buffer bytes.Buffer
	count := len(variants)
	if count > 0 {
		for i, variant := range variants {
			buffer.WriteString(fmt.Sprintf("(%d,'%s',0,%d)", questionId, sanitizeString(variant), i))
			if i < count-1 {
				buffer.WriteString(",")
			}
		}

		query := fmt.Sprintf("INSERT INTO variants (question_id, text, votes_count, index_number) VALUES %s", buffer.String())
		database.execQuery(query)
	}
}

func (database *Database) AddQuestionAnswer(questionId int64, userId int64, index int64) {
	database.execQuery(fmt.Sprintf("INSERT INTO answered_questions (user_id, question_id) VALUES (%d,%d)", userId, questionId))

	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK variants SET votes_count=votes_count+1 WHERE question_id=%d AND index_number=%d", questionId, index))
}

func (database *Database) RemoveUserPendingQuestion(userId int64, questionId int64) {
	database.execQuery(fmt.Sprintf("DELETE FROM pending_questions WHERE user_id=%d AND question_id=%d", userId, questionId))
}

func (database *Database) GetQuestionRespondents(questionId int64) (respondents []int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT u.chat_id FROM answered_questions as q INNER JOIN users as u WHERE q.question_id=%d AND q.user_id=u.id", questionId))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var respondent int64
		err := rows.Scan(&respondent)
		if err != nil {
			log.Fatal(err.Error())
		}
		respondents = append(respondents, respondent)
	}

	return
}

func (database *Database) GetReadyUsersChatIds() (users []int64) {
	rows, err := database.conn.Query("SELECT chat_id FROM users WHERE is_ready=1")
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		var chatId int64
		err := rows.Scan(&chatId)
		if err != nil {
			log.Fatal(err.Error())
		}
		users = append(users, chatId)
	}

	return
}

func (database *Database) GetAllUsersChatIds() (chatIds []int64) {
	rows, err := database.conn.Query("SELECT chat_id FROM users")
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var chatId int64
		err := rows.Scan(&chatId)
		if err != nil {
			log.Fatal(err.Error())
		}
		chatIds = append(chatIds, chatId)
	}

	return
}

func (database *Database) StartCreatingQuestion(author int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK users SET is_ready=0 WHERE id=%d", author))
	database.createUniqueRecord("questions", fmt.Sprintf("NULL,%d,NULL,0,NULL,NULL,NULL", author))
}

func (database *Database) IsQuestionReady(questionId int64) (isReady bool) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM questions WHERE id=%d AND text NOT NULL AND end_time NOT NULL AND min_votes NOT NULL AND max_votes NOT NULL", questionId))
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	if rows.Next() {
		var count int64
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
		}

		if count != 0 {
			isReady = true

			if count != 1 {
				log.Fatalf("Count should be 0 or 1: %d", count)
			}
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No row found")
	}

	return
}

func (database *Database) CommitQuestion(questionId int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK questions SET status=1 WHERE id=%d", questionId))

	// add to pending questions for all users
	database.conn.Exec(fmt.Sprintf("INSERT INTO pending_questions (user_id, question_id) "+
		"SELECT DISTINCT id, %d FROM users;", questionId))
}

func (database *Database) DiscardQuestion(questionId int64) {
	database.execQuery(fmt.Sprintf("DELETE FROM questions where status=0 AND id=%d", questionId))
}

func (database *Database) FinishQuestion(questionId int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK questions SET status=2 WHERE id=%d", questionId))
}

func (database *Database) MarkUserReady(userId int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK users SET is_ready=1 WHERE id=%d", userId))
}

func (database *Database) UnmarkUserReady(userId int64) {
	database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK users SET is_ready=0 WHERE id=%d", userId))
}

func (database *Database) UnmarkUsersReady(chatIds []int64) {
	count := len(chatIds)
	if count > 0 {
		var buffer bytes.Buffer
		for i, chatId := range chatIds {
			buffer.WriteString(strconv.FormatInt(chatId, 10))
			if i < count-1 {
				buffer.WriteString(",")
			}
		}

		database.execQuery(fmt.Sprintf("UPDATE OR ROLLBACK users SET is_ready=0 WHERE chat_id IN (%s)", buffer.String()))
	}
}

func (database *Database) RemoveQuestionFromAllUsers(questionId int64) {
	database.execQuery(fmt.Sprintf("DELETE FROM pending_questions WHERE question_id=%d", questionId))
}

func (database *Database) GetUsersAnsweringQuestionNow(questionId int64) (users []int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT t.user_id FROM"+
		" (SELECT user_id, MIN(question_id) as next_question_id FROM pending_questions GROUP BY user_id) as t"+
		" WHERE t.next_question_id=%d", questionId))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var user int64
		err := rows.Scan(&user)
		if err != nil {
			log.Fatal(err.Error())
		}
		users = append(users, user)
	}

	return
}

func (database *Database) GetQuestionPendingCount(questionId int64) (count int) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT count(*) FROM pending_questions WHERE question_id=%d", questionId))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No question found")
	}

	return
}

func (database *Database) IsQuestionHasText(questionId int64) (hasText bool) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM questions WHERE id=%d AND text NOT NULL", questionId))
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	if rows.Next() {
		var count int64
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
		}

		if count != 0 {
			hasText = true

			if count != 1 {
				log.Fatalf("Count should be 0 or 1: %d", count)
			}
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No row found")
	}

	return
}

func (database *Database) IsQuestionHasRules(questionId int64) (hasRules bool) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM questions WHERE id=%d AND end_time NOT NULL AND min_votes NOT NULL AND max_votes NOT NULL", questionId))
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	if rows.Next() {
		var count int64
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
		}

		if count != 0 {
			hasRules = true

			if count != 1 {
				log.Fatalf("Count should be 0 or 1: %d", count)
			}
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No row found")
	}

	return
}

func (database *Database) GetActiveQuestions() (activeQuestions []int64) {
	rows, err := database.conn.Query("SELECT id FROM questions WHERE status=1")

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var questionId int64
		err := rows.Scan(&questionId)
		if err != nil {
			log.Fatal(err.Error())
		}
		activeQuestions = append(activeQuestions, questionId)
	}

	return
}

func (database *Database) InitNewUserQuestions(userId int64) {
	// add to the user all unfinished questions that wasn't answered and already not in pending questions
	// of this user
	database.execQuery(fmt.Sprintf("INSERT INTO pending_questions (user_id, question_id)"+
		" SELECT %d, q.id FROM questions as q"+
		" LEFT JOIN pending_questions as pq ON q.id=pq.question_id AND pq.user_id=%d"+
		" LEFT JOIN answered_questions as aq ON q.id=aq.question_id AND aq.user_id=%d"+
		" WHERE pq.user_id IS NULL AND aq.user_id IS NULL AND q.status=1", userId, userId, userId))
}

func (database *Database) GetLastFinishedQuestions(count int) (questions []int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM"+
		"(SELECT q.id as id FROM questions as q"+
		" WHERE q.status=2"+
		" ORDER BY q.id DESC LIMIT %d) ORDER BY id ASC", count))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var questionId int64
		err := rows.Scan(&questionId)
		if err != nil {
			log.Fatal(err.Error())
		}
		questions = append(questions, questionId)
	}

	return
}

func (database *Database) GetDatabaseVersion() (version string) {
	rows, err := database.conn.Query("SELECT string_value FROM global_vars WHERE name=\"version\"")

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&version)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		// that means it's a new clean database
		version = latestVersion
	}

	return
}

func (database *Database) SetDatabaseVersion(version string) {
	database.execQuery("DELETE FROM global_vars WHERE name='version'")
	database.execQuery(fmt.Sprintf("INSERT INTO global_vars (name, string_value) VALUES ('version', '%s')", sanitizeString(version)))
}

func (database *Database) IsUserBanned(userId int64) (isBanned bool) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT COUNT(*) FROM users WHERE id=%d AND banned=1", userId))

	if err != nil {
		log.Fatal(err.Error())
		return
	}
	defer rows.Close()

	if rows.Next() {
		var count int64
		err := rows.Scan(&count)
		if err != nil {
			log.Fatal(err.Error())
		}
		if count != 0 {
			isBanned = true

			if count != 1 {
				log.Fatalf("Count should be 0 or 1: %d", count)
			}
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal("No row found")
	}

	return
}

func (database *Database) BanUser(userId int64) {
	database.execQuery(fmt.Sprintf("UPDATE users SET banned=1 where id=%d", userId))
}

func (database *Database) GetLastPublishedQuestions(count int64) (questions []int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM"+
		"(SELECT q.id as id FROM questions as q"+
		" WHERE q.status=1 OR q.status=2"+
		" ORDER BY q.id DESC LIMIT %d) ORDER BY id ASC", count))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var questionId int64
		err := rows.Scan(&questionId)
		if err != nil {
			log.Fatal(err.Error())
		}
		questions = append(questions, questionId)
	}

	return
}

func (database *Database) RemoveQuestion(questionId int64) {
	database.execQuery(fmt.Sprintf("DELETE FROM questions WHERE id=%d", questionId))
}

func (database *Database) GetAuthor(questionId int64) (author int64, findErr error) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT author FROM questions WHERE id=%d", questionId))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&author)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
		findErr = errors.New("No question found")
	}

	return
}

func (database *Database) GetUserLastQuestions(userId int64, count int) (questions []int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM"+
		"(SELECT q.id as id FROM questions as q"+
		" WHERE q.author=%d AND (q.status=1 OR q.status=2)"+
		" ORDER BY q.id DESC LIMIT %d) ORDER BY id ASC", userId, count))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var questionId int64
		err := rows.Scan(&questionId)
		if err != nil {
			log.Fatal(err.Error())
		}
		questions = append(questions, questionId)
	}

	return
}

func (database *Database) GetUserLastFinishedQuestions(userId int64, count int) (questions []int64) {
	rows, err := database.conn.Query(fmt.Sprintf("SELECT id FROM"+
		"(SELECT q.id as id FROM questions as q"+
		" WHERE q.author=%d AND q.status=2"+
		" ORDER BY q.id DESC LIMIT %d) ORDER BY id ASC", userId, count))

	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var questionId int64
		err := rows.Scan(&questionId)
		if err != nil {
			log.Fatal(err.Error())
		}
		questions = append(questions, questionId)
	}

	return
}
