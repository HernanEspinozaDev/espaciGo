import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { AccountPrivacyFacade } from './application/account-privacy-facade';

@Component({
  providers: [AccountPrivacyFacade],
  selector: 'app-account-privacy-page',
  imports: [UiButton, RouterLink],
  templateUrl: './account-privacy-page.html',
  styleUrl: './account-privacy-page.scss',
})
export class AccountPrivacyPage {
  private readonly facade = inject(AccountPrivacyFacade);
  get session() {
    return this.facade.session;
  }
  get profile() {
    return this.facade.profile;
  }
  get profileReady() {
    return this.facade.profileReady;
  }
  get displayName() {
    return this.facade.displayName;
  }
  get phone() {
    return this.facade.phone;
  }
  get photo() {
    return this.facade.photo;
  }
  get photoUrl() {
    return this.facade.photoUrl;
  }
  get payout() {
    return this.facade.payout;
  }
  get rights() {
    return this.facade.rights;
  }
  get busy() {
    return this.facade.busy;
  }
  get savingProfile() {
    return this.facade.savingProfile;
  }
  get error() {
    return this.facade.error;
  }
  get notice() {
    return this.facade.notice;
  }
  get exportNotice() {
    return this.facade.exportNotice;
  }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  ngOnDestroy(): void {
    this.facade.ngOnDestroy();
  }
  readonly load = this.facade.load.bind(this.facade);
  readonly saveProfile = this.facade.saveProfile.bind(this.facade);
  readonly createPhoto = this.facade.createPhoto.bind(this.facade);
  readonly uploadPhoto = this.facade.uploadPhoto.bind(this.facade);
  readonly isSyntheticPhoto = this.facade.isSyntheticPhoto.bind(this.facade);
  get photoUploadProgress() { return this.facade.photoUploadProgress; }
  readonly removePhoto = this.facade.removePhoto.bind(this.facade);
  readonly createPayout = this.facade.createPayout.bind(this.facade);
  readonly removePayout = this.facade.removePayout.bind(this.facade);
  readonly requestRight = this.facade.requestRight.bind(this.facade);
  readonly downloadJson = this.facade.downloadJson.bind(this.facade);
  readonly downloadZip = this.facade.downloadZip.bind(this.facade);
  readonly formatDate = this.facade.formatDate.bind(this.facade);
}
