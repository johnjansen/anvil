#!/usr/bin/env python3
import pika
import sys

# Connect to RabbitMQ server
connection = pika.BlockingConnection(pika.ConnectionParameters('localhost'))
channel = connection.channel()

# Declare the queue (this will create it if it doesn't exist)
channel.queue_declare(queue='test-queue', durable=True)

# Publish a message
message = 'Hello from Python!'
channel.basic_publish(exchange='', routing_key='test-queue', body=message)

print(f"Sent message: {message}")

# Close the connection
connection.close()