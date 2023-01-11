
# Kafka Otp Email

Running kafka cluster using docker compose 

    docker-compose up -d
After kafka cluster running copy **.env.example**  to **.env** and run command

    go mod tidy`

After  install depedency  run consumer with command

    go run otp/main.go
And than run the server with command 

    go run server/main.go

