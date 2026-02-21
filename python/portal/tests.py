"""Tests for core PbT prototype behavior."""

from django.contrib.auth.models import User
from django.test import TestCase

from .models import ContactRequest, MemberProfile
from .ttt import evaluate_answers


class TttEvaluationTests(TestCase):
    def test_tie_preferences_follow_spec(self):
        answers = {}
        choices = (["E", "I"] * 5) + (["S", "N"] * 5) + (["T", "F"] * 5) + (["J", "P"] * 5)
        for i, choice in enumerate(choices):
            answers[i] = choice
        result = evaluate_answers(answers)
        self.assertIsNotNone(result)
        self.assertEqual(result["ttt_type"][0], "I")
        self.assertEqual(result["ttt_type"][1], "S")
        self.assertEqual(result["ttt_type"][2], "T")
        self.assertEqual(result["ttt_type"][3], "J")


class RcdFlowTests(TestCase):
    def setUp(self):
        self.a = User.objects.create_user("alice", password="pw")
        self.b = User.objects.create_user("bob", password="pw")
        MemberProfile.objects.create(user=self.a, town="A", country="X", motto1="m1")
        MemberProfile.objects.create(user=self.b, town="B", country="X", motto1="m2")

    def test_send_accept_reject_paths(self):
        self.client.login(username="alice", password="pw")
        self.client.post("/rcd/", {"members": ["bob"], "action": "send"})
        self.assertTrue(ContactRequest.objects.filter(from_member=self.a, to_member=self.b).exists())

        self.client.logout()
        self.client.login(username="bob", password="pw")
        self.client.post("/rcd/", {"members": ["alice"], "action": "reject"})
        self.assertFalse(ContactRequest.objects.filter(from_member=self.a, to_member=self.b).exists())

        self.client.logout()
        self.client.login(username="alice", password="pw")
        self.client.post("/rcd/", {"members": ["bob"], "action": "send"})

        self.client.logout()
        self.client.login(username="bob", password="pw")
        self.client.post("/rcd/", {"members": ["alice"], "action": "accept"})
        self.assertTrue(ContactRequest.objects.filter(from_member=self.a, to_member=self.b).exists())
        self.assertTrue(ContactRequest.objects.filter(from_member=self.b, to_member=self.a).exists())
