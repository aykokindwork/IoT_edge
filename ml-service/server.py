import grpc
import joblib
import numpy as np
from concurrent import futures
import classifier_pb2
import classifier_pb2_grpc
import warnings
import os

warnings.filterwarnings("ignore", category=UserWarning)

# Загружаем модель и метаданные
model = joblib.load('models/ciciot_ml_model.pkl')
feature_names = joblib.load('models/ciciot_features.pkl')
thresholds = joblib.load('models/ciciot_thresholds.pkl')


LOW_T = float(os.getenv('LOW_THRESHOLD', thresholds['LOW_T']))
HIGH_T = float(os.getenv('HIGH_THRESHOLD', thresholds['HIGH_T']))

print(f"Model loaded. Features: {feature_names}")
print(f"ACTIVE THRESHOLDS: LOW={LOW_T}, HIGH={HIGH_T}") # Чтобы видеть в логах, что подцепилось


class ClassifierServicer(classifier_pb2_grpc.ClassifierServiceServicer):

    def Classify(self, request, context):
        features = np.array(request.features, dtype=np.float32).reshape(1, -1)

        proba = model.predict_proba(features)[0][1]  # вероятность класса Attack

        if proba >= HIGH_T:
            verdict = "attack"
        elif proba <= LOW_T:
            verdict = "benign"
        else:
            verdict = "suspicious"

        print(f"[{request.flow_id}] proba={proba:.4f} verdict={verdict}")

        return classifier_pb2.ClassifyResponse(
            probability=float(proba),
            verdict=verdict,
        )


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    classifier_pb2_grpc.add_ClassifierServiceServicer_to_server(
        ClassifierServicer(), server
    )
    server.add_insecure_port('[::]:50051')
    server.start()
    print("ML service listening on port 50051")
    server.wait_for_termination()


if __name__ == '__main__':
    serve()