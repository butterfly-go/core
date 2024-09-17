package store

func Init() error {
	err := InitRedis()
	if err != nil {
		return err
	}
	err = InitSqlDB()
	if err != nil {
		return err
	}
	return InitMongo()
}
