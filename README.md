IoT Edge Intrusion Detection System (IDS) Gateway

Прототип интеллектуального граничного шлюза для защиты сетей IoT на основе анализа сетевых потоков (flows) в реальном времени.
Архитектура системы

Система построена по принципу Edge Computing (модель Cisco) и состоит из трех уровней:

    Orchestrator (Go): Высокопроизводительный сервис для захвата трафика (gopacket), ведения статистики потоков и оркестрации решений.

    ML-Worker (Python): Микросервис, реализующий бинарную классификацию на базе модели RandomForestClassifier (Scikit-learn). Связь осуществляется через gRPC.

    Data Bus (Kafka): Очередь сообщений для передачи подозрительных потоков в облачный сегмент для глубокого анализа.

Технологический стек

    Backend: Go 1.24+ (gopacket, grpc-go, kafka-go)

    ML: Python 3.12 (scikit-learn, joblib, grpcio)

    Infra: Docker, Docker Compose, Kafka (Confluent Platform)

Реализованные возможности

    Zero-copy packet capture: Использование буферов сетевой карты для минимизации нагрузки на CPU.

    Online Flow Tracking: Ведение статистики потоков в памяти с фиксированным потреблением RAM (O(1)).

    Low-Latency gRPC: Бинарный протокол связи между Go и Python для принятия решений за миллисекунды.

    Asynchronous Buffering: Неблокирующая отправка данных в Kafka для защиты основного процесса обработки трафика.

Как запустить
Требования

    Docker и Docker Compose

    (Для Windows) Установленный Npcap для работы сетевого интерфейса в режиме host.

Запуск
code Bash

# Сборка и запуск всей инфраструктуры в фоне
docker-compose up -d --build

Мониторинг логов
gateway:
# Просмотр решений шлюза
    docker-compose logs -f gateway

ml-model:
# Просмотр работы ML-модели
    docker-compose logs -f ml-service

Проверка данных в KafkaL:

    docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic suspicious-flows --from-beginning

Контракт признаков (Features)

Модель анализирует 10 признаков для каждого потока:

    flow_duration, 2. Protocol Type, 3. Duration, 4. Tot sum, 5. Min, 6. Max, 7. AVG, 8. Tot size, 9. IAT, 10. Number.

Дизайн решений

    Benign (prob < 0.35): Трафик пропускается.

    Suspicious (0.35 - 0.75): Данные отправляются в Kafka для анализа в облаке.

    Attack (prob > 0.75): Имитация блокировки IP адреса (IPS mode).