package chat

import (
	"github.com/RicheyJang/PaimengBot/manager"
	"github.com/RicheyJang/PaimengBot/utils"
	log "github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot/message"
	"gorm.io/gorm/clause"
)

func SetDialogue(groupID int64, question string, answer message.Message) error {
	groupD := GroupChatDialogue{
		GroupID:  groupID,
		Question: question,
		Answer:   utils.JsonString(answer),
	}
	return proxy.GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group_id"}, {Name: "question"}},
		UpdateAll: true,
	}).Create(&groupD).Error
}

func DeleteDialogue(groupID int64, question string) error {
	groupD := GroupChatDialogue{
		GroupID:  groupID,
		Question: question,
	}
	return proxy.GetDB().Delete(&groupD).Error
}

func GetDialogue(groupID int64, question string) message.Message {
	resD := GroupChatDialogue{}
	rows := proxy.GetDB().Where(&GroupChatDialogue{
		GroupID:  groupID,
		Question: question,
	}).Find(&resD).RowsAffected
	if rows == 0 {
		return nil
	}
	return message.ParseMessage([]byte(resD.Answer))
}

func GetAllQuestion(groupID int64) []string {
	var resD []GroupChatDialogue
	proxy.GetDB().Where("group_id = ?", groupID).Or("group_id = ?", 0).Find(&resD)
	var qs []string
	for _, r := range resD {
		qs = append(qs, r.Question)
	}
	return utils.MergeStringSlices(qs)
}

type GroupChatDialogue struct {
	GroupID  int64  `gorm:"column:group_id;primaryKey;autoIncrement:false"`
	Question string `gorm:"column:question;primaryKey;autoIncrement:false"`
	Answer   string `gorm:"column:answer"`
}

func init() {
	err := manager.GetDB().AutoMigrate(&GroupChatDialogue{})
	if err != nil {
		log.Errorf("[SQL] GroupChatDialogue 初始化失败, err: %v", err)
	}
}
