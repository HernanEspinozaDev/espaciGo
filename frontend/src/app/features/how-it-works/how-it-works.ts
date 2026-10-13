import { UiButton } from '../../shared/ui/button/button';
import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  imports: [UiButton, RouterLink],
  selector: 'app-how-it-works',
  styleUrl: './how-it-works.scss',
  templateUrl: './how-it-works.html',
})
export class HowItWorks {}
