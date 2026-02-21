"""Data model for the PbT portal."""

from django.contrib.auth.models import User
from django.db import models


class MemberProfile(models.Model):
    """Registration and status data stored for each member."""

    ENNEAGRAM_CHOICES = [(0, "unknown")] + [(i, f"{i}") for i in range(1, 10)] + [(10, "none")]

    user = models.OneToOneField(User, on_delete=models.CASCADE)
    town = models.CharField(max_length=120)
    country = models.CharField(max_length=120)
    motto1 = models.CharField(max_length=255)
    motto2 = models.CharField(max_length=255, blank=True)
    likes = models.TextField(blank=True)
    dislikes = models.TextField(blank=True)
    gps_coordinates = models.CharField(max_length=100, blank=True)
    enneagramtype1 = models.IntegerField(choices=ENNEAGRAM_CHOICES, default=0)
    enneagramtype2 = models.IntegerField(choices=ENNEAGRAM_CHOICES, default=10)
    registration_time = models.DateTimeField(auto_now_add=True)

    def likes_list(self):
        return [x.strip() for x in self.likes.split(",") if x.strip()]

    def dislikes_list(self):
        return [x.strip() for x in self.dislikes.split(",") if x.strip()]


class TttResult(models.Model):
    """Latest TTT result for one member."""

    user = models.OneToOneField(User, on_delete=models.CASCADE)
    result = models.CharField(max_length=64)
    ttt_type = models.CharField(max_length=4)
    temperament = models.CharField(max_length=2)
    ei = models.IntegerField(default=0)
    sn = models.IntegerField(default=0)
    tf = models.IntegerField(default=0)
    jp = models.IntegerField(default=0)
    taken_at = models.DateTimeField(auto_now=True)


class ContactRequest(models.Model):
    """One directed RCD relation from from_member to to_member."""

    from_member = models.ForeignKey(User, on_delete=models.CASCADE, related_name="rcd_sent")
    to_member = models.ForeignKey(User, on_delete=models.CASCADE, related_name="rcd_received")
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        constraints = [
            models.UniqueConstraint(fields=["from_member", "to_member"], name="unique_contact_request"),
        ]
