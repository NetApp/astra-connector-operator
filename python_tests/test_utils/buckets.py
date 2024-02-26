from minio import Minio
from minio.error import S3Error
from python_tests.log import logger


class Bucket:
    def __init__(self, bucket_name, client, host, access_key, secret_key):
        self.bucket_name = bucket_name
        self.host = host
        self.access_key = access_key
        self.secret_key = secret_key
        self.client = client

    def read(self, object_path):  # todo read() not tested yet
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
        logger.info(f"Initializing BucketManager for S3 host {host}")
        self.client = Minio(self.host,
                            access_key=self.access_key,
                            secret_key=self.secret_key,
                            secure=False)

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

    def delete_bucket(self, bucket_name):
        self.client.remove_bucket(bucket_name)

    def delete_object(self, bucket_name, object_name):
        self.client.remove_object(bucket_name, object_name)

    # todo: hitting bucket delete err due to existing objects, nested files may not be getting deleted
    def cleanup_buckets(self):
        logger.info(f"Cleaning up buckets...")
        for bucket in self.created_buckets:
            # List all objects in the bucket and delete them
            bucket = bucket.bucket_name
            try:
                objects = self.client.list_objects(bucket)
                for obj in objects:
                    logger.info(f"Deleting object {obj.object_name} in bucket {bucket}")
                    self.client.remove_object(bucket, obj.object_name)

                # Now that the bucket is empty, delete the bucket
                logger.info(f"Deleting bucket {bucket}")

                self.delete_bucket(bucket)
            except S3Error as err:
                if err.code == 'NoSuchBucket':
                    logger.info(f"Bucket {bucket} already removed, continuing cleanup")
