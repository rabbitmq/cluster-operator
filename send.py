#!/usr/bin/env python
import pika
import ssl

context = ssl.create_default_context(cafile='./ca.crt')
ssl_options = pika.SSLOptions(context, 'rabbitmqcluster-sample-rabbitmq-ingress')
creds = pika.credentials.PlainCredentials("<rabbit admin username>", "<rabbit admin password>", erase_on_connect=False)
connection = pika.BlockingConnection(
    pika.ConnectionParameters(
        credentials=creds,
        port=5671,
        host="rabbitmqcluster-sample-rabbitmq-ingress",
        ssl_options=ssl_options
        ))
channel = connection.channel()

channel.queue_declare(queue='hello')

channel.basic_publish(exchange='', routing_key='hello', body='Hello World!')
print(" [x] Sent 'Hello World!'")
connection.close()
