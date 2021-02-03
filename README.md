## Как запустить и отлаживать бота

Требуется docker-compose версии не старше 1.28.

Чтобы запустить бот в докере локально:

```shell
TOKEN=token.txt make compose-all
```

Или удалённо на хосте `yourhost`, куда вы можете зайти по `ssh` под пользователем `youruser`:

```shell
DOCKER_HOST="ssh://youruser@yourhost" TOKEN="/token.txt" make compose-all
```

Токен нужно предварительно скопировать на удалённый хост и затем указать путь к нему в переменной `TOKEN`.

К боту в докере можно подсоединиться отладчиком. Для этого нужно запустить бота

```shell
TOKEN=token.txt make compose-all-debug
```

После этого delve будет доступен через порт 40000.

Чтобы отлаживать в IDEA или Goland, нужно создать конфигурацию "Go remote"
и указать "Host: localhost", "Port: 40000"

Тесты можно запустить с помощью `make test`.

## Как запустить код вне докера

Инструкция, чтобы запустить бота или тесты локально

* Файл с токеном для бота ожидается в первом аргументе командной строки

* Бот также ожидает работающую Mongo по адресу `mongodb://mongo:27017`. Этот адрес можно заменить, добавив флаг сборки,
  например

```
-ldflags "-X yandexschooldating/config.MongoUri=mongodb://localhost:27017"
```

Так можно запустить Mongo для тестов без сохранения состояния

```shell
docker run -p 27017:27017 --detach mongo
```

* Чтобы работали тесты, отладка и coverage в IDEA или Goland, нужно в "Edit configurations" добавить "Go tool arguments"

```
-ldflags "-X yandexschooldating/config.MongoUri=mongodb://localhost:27017" -gcflags="all=-N -l"
```
