import { ProfileCard } from "@/features/account/ui/profile-card";
import { SyntheticPhotoCard } from "@/features/account/ui/synthetic-photo-card";
import { SyntheticPayoutCard } from "@/features/account/ui/synthetic-payout-card";

export default function MiCuentaPage() {
  return (
    <div className="space-y-8">
      <ProfileCard />
      <SyntheticPhotoCard />
      <SyntheticPayoutCard />
    </div>
  );
}
