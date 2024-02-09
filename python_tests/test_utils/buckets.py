from minio import Minio
from python_tests.log import logger


class Bucket:
    def __init__(self, bucket_name, client, host, access_key, secret_key):
        self.bucket_name = bucket_name
        self.host = host
        self.access_key = access_key
        self.secret_key = secret_key
        self.client = client

    def read(self, object_path):  # todo not tested
        data = self.client.get_object(self.bucket_name, object_path)
        # Read the data
        file_data = data.read()
        return file_data


class BucketManager:

    created_buckets: list[Bucket] = []

    def __init__(self, host, access_key, secret_key):
        self.host = host
        self.access_key = access_key
        self.secret_key = secret_key
        self.client = Minio(self.host,
                            access_key=self.access_key,
                            secret_key=self.secret_key,
                            secure=True)

    def create_bucket(self, bucket_name):
        bucket = Bucket(
            bucket_name=bucket_name,
            client=self.client,
            access_key=self.access_key,
            secret_key=self.secret_key,
            host=self.host
        )
        logger.info(f"Creating bucket {bucket_name}")
        self.client.make_bucket(bucket_name)
        self.created_buckets.append(bucket)
        return bucket

    def delete(self, bucket_name):
        self.client.remove_bucket(bucket_name)

    def cleanup_buckets(self):
        logger.info(f"Cleaning up buckets...")
        for bucket in self.created_buckets:
            logger.info(f"Deleting bucket {bucket.bucket_name}")
            self.delete(bucket.bucket_name)
