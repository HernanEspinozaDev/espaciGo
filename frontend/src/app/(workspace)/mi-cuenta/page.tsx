import { ProfileCard } from "@/features/account/ui/profile-card";
import { AccountSpacesCard } from "@/features/host-listings/ui/account-spaces-card";
import { SyntheticPhotoCard } from "@/features/account/ui/synthetic-photo-card";
import { SyntheticPayoutCard } from "@/features/account/ui/synthetic-payout-card";

export default function MiCuentaPage() {
  return (
    <div className="space-y-8">
      <AccountSpacesCard />
      <ProfileCard />
      <SyntheticPhotoCard />
      <SyntheticPayoutCard />
    </div>
  );
}
