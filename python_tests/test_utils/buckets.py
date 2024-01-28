from minio import Minio
from minio.error import S3Error


class Bucket:
    def __init__(self, bucket_name, host, access_key, secret_key):
        self.host = host
        self.bucket_name = bucket_name
        self.access_key = access_key
        self.secret_key = secret_key
        self.s3_client = Minio(
            self.host,
            access_key=self.access_key,
            secret_key=self.secret_key,
        )
        self.s3_client.make_bucket(self.bucket_name)

