from datetime import datetime, timedelta
from requests import Session
from urllib.parse import urljoin, urlencode

class LokiClient:
    def __init__(self, base_url: str, session: Session = None):
        self.session = session or Session()
        self.base_url = base_url.rstrip('/') + '/'
        self._normalize = lambda v: str(v).strip("'\"") if v else v

    def _build_query(self, start: datetime = None, end: datetime = None, matchers: str = ""):
        params = {}
        if start:
            params['start'] = start.strftime("%Y-%m-%dT%H:%M:%S.%fZ")
        if end:
            params['end'] = end.strftime("%Y-%m-%dT%H:%M:%S.%fZ")
        if matchers:
            params['match[]'] = matchers
        return '&'.join(f"{k}={v}" for k, v in params.items())

    def get_label_names(self, start: datetime = None, end: datetime = None, matchers: str = "") -> list:
        url = urljoin(self.base_url, 'labels')
        query = self._build_query(start, end, matchers)
        if query:
            url += f'?{query}'
        resp = self.session.get(url)
        return resp.json() if resp.status_code == 200 else []

    def get_label_values(self, name: str, start: datetime = None, end: datetime = None, matchers: str = "") -> list:
        path = f"label/{name}/values"
        url = urljoin(self.base_url, path)
        query = self._build_query(start, end, matchers)
        if query:
            url += f'?{query}'
        resp = self.session.get(url)
        return resp.json() if resp.status_code == 200 else []

    def get_series(self, start: datetime = None, end: datetime = None, matchers: str = "") -> dict:
        url = urljoin(self.base_url, 'series')
        query = self._build_query(start, end, matchers)
        if query:
            url += f'?{query}'
        resp = self.session.get(url)
        return resp.json() if resp.status_code == 200 else {}

if __name__ == "__main__":
    base = "http://localhost:3101"
    client = LokiClient(base)
    labels = client.get_label_names()
    for label in labels:
        values = client.get_label_values(label, start=datetime.utcnow(), end=datetime.utcnow() + timedelta(hours=1))
        print(f"{label} -> {values}")