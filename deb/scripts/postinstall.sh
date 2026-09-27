#!/bin/bash
systemctl daemon-reload
systemctl enable laserlabel
systemctl restart laserlabel
