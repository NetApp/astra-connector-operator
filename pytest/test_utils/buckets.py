from minio import Minio
from minio.error import S3Error


class Bucket:
    def __init__(self, name, s3_client):
        self.name = name
        self.s3_client = s3_client


class BucketManager:
    def __init__(self, host, access_key, secret_key):
        self.host = host
        self.access_key = access_key
        self.secret_key = secret_key
        self.s3_client = Minio(
            self.host,
            access_key=self.access_key,
            secret_key=self.secret_key,
        )

    def create_bucket(self, bucket_name) -> Bucket:
        self.s3_client.make_bucket(bucket_name)
        return Bucket(
            name=bucket_name,
            s3_client=self.s3_client
        )



