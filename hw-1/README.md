// принять заказ от курьера
curl -X POST http://localhost:9000/orders \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "id": "ORDER1",
    "recipient_id": "USER1",
    "wrapper_type": "Box",
    "price": 1000,
    "weight": 5.5,
    "storage_until": "2025-12-31",
    "with_additional_membrane": true
  }'

// вернуть заказ курьеру если срок истек
curl -X DELETE http://localhost:9000/orders/ORDER1 \
  -u admin:password

  

// выдать юзеру заказ
curl -X POST http://localhost:9000/process/issue \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "USER1",
    "order_ids": ["ORDER1", "ORDER2"]
  }'

// принять возврат от юзера
curl -X POST http://localhost:9000/process/return \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "USER1",
    "order_ids": ["ORDER1", "ORDER2"]
  }'

// получение списка заказов
curl -X GET http://localhost:9000/users/USER1/orders \
  -u admin:password

curl -X GET "http://localhost:9000/users/USER1/orders?active=true" \
  -u admin:password

curl -X GET "http://localhost:9000/users/USER1/orders?last=5" \
  -u admin:password

curl -X GET "http://localhost:9000/users/USER1/orders?active=true&last=5" \
  -u admin:password


curl -X GET http://localhost:9000/returns \
  -u admin:password

curl -X GET "http://localhost:9000/returns?page=2&limit=20" \
  -u admin:password


история 
curl -X GET http://localhost:9000/orders/ORDER1/history \
  -u admin:password