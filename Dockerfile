FROM eclipse-temurin:21-jre-alpine

WORKDIR /app
COPY build/libs/*.jar ./

ENV FILECONDUIT_SECRET_HASH=""

EXPOSE 8080

CMD java -cp "*" it.germanorizzo.proj.fileconduit.Main