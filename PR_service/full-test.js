import http from 'k6/http';
import { sleep, check, group } from 'k6';

export let options = {
    stages: [
        { duration: '10s', target: 10 }, // 10 пользователей
        { duration: '20s', target: 20 }, // 20 пользователей
        { duration: '10s', target: 0 },  // спадаем
    ],
    thresholds: {
        http_req_duration: ['p(95)<300'],  // SLA
        http_req_failed: ['rate<0.01'],    // не более 1% ошибок
    },
};

const BASE = "http://localhost:8080";

function randomId(prefix) {
    return prefix + "-" + Math.floor(Math.random() * 1e9);
}

export default function () {
    //
    // 1. Создание команды
    //
    group("team creation", () => {
        const teamId = randomId("team");
        const user1 = randomId("u");
        const user2 = randomId("u");
        const user3 = randomId("u");

        const payload = JSON.stringify({
            team_name: teamId,
            members: [
                { user_id: user1, username: `${teamId}-alice`, is_active: true },
                { user_id: user2, username: `${teamId}-bob`, is_active: true },
                { user_id: user3, username: `${teamId}-charlie`, is_active: true },
            ]
        });

        let res = http.post(`${BASE}/team/add`, payload, {
            headers: { "Content-Type": "application/json" }
        });

        check(res, {
            "team created (201)": r => r.status === 201,
        });

        //
        // 2. Создание PR (должен автоматически назначить двух ревьюеров)
        //
        group("create PR", () => {
            const prId = randomId("pr");

            const prPayload = JSON.stringify({
                pull_request_id: prId,
                pull_request_name: "Big refactoring",
                author_id: user1,
            });

            let create = http.post(`${BASE}/pullRequest/create`, prPayload, {
                headers: { "Content-Type": "application/json" }
            });

            check(create, {
                "PR created (201)": r => r.status === 201,
                "latency < 300ms": r => r.timings.duration < 300,
            });

            //
            // 3. Reassign одного ревьюера (конкурентная логика)
            //
            const reassignPayload = JSON.stringify({
                pull_request_id: prId,
            });

            const reass = http.post(`${BASE}/pullRequest/reassign`, reassignPayload, {
                headers: { "Content-Type": "application/json" }
            });

            check(reass, {
                "reassign valid (200 or 400)": r => [200, 400].includes(r.status),
            });

            //
            // 4. Merge PR (идемпотентный)
            //
            const mergePayload = JSON.stringify({
                pull_request_id: prId,
            });

            const m1 = http.post(`${BASE}/pullRequest/merge`, mergePayload, {
                headers: { "Content-Type": "application/json" }
            });

            const m2 = http.post(`${BASE}/pullRequest/merge`, mergePayload, {
                headers: { "Content-Type": "application/json" }
            });

            check(m1, { "merge OK (200)": r => r.status === 200 });
            check(m2, { "merge idempotent (200)": r => r.status === 200 });

            //
            // 5. Получение списка PR для одного пользователя
            //
            const getPRs = http.get(`${BASE}/users/getReview?user_id=${user2}`);

            check(getPRs, {
                "get PRs OK (200)": r => r.status === 200,
            });
        });
    });

    sleep(0.2);
}

